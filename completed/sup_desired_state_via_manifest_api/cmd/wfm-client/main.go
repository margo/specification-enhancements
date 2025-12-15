package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"skeleton/pkg/common"

	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v3"
)

var (
	digestRe = regexp.MustCompile(`^[a-z0-9_\-]+:[0-9a-f]{64}$`)
	verbose  bool
)

type clientConfig struct {
	BaseURL      string
	DeviceID     string
	PollInterval time.Duration
}

// This struct holds the latest manifest and deployment state fetched from the server.
// Production code would persist this state.
type state struct {
	ManifestETag    string
	ManifestVersion uint64
	Deployments     map[string]deploymentCacheEntry // deploymentId -> cache entry
	BundleFetched   bool
}

type deploymentCacheEntry struct {
	Digest        string
	ApplicationID string
	Name          string
}

type resolvedDeployment struct {
	Digest     string
	Descriptor *common.ApplicationDeploymentDescriptor
}

func main() {
	cmd := &cli.Command{
		Usage: "Workload Fleet Management API Client",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "wfm-base-url", Value: "http://localhost:8080", Usage: "Base URL of WFM API server"},
			&cli.StringFlag{Name: "device-id", Value: "c92cb339-c99c-4eca-9dd4-f8484dd16cfb", Usage: "Device identifier"},
			&cli.DurationFlag{Name: "poll-interval", Value: 30 * time.Second, Usage: "Polling interval for manifest"},
			&cli.BoolFlag{Name: "verbose", Usage: "Enable verbose logging"},
		},
		Action: run,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	cfg := clientConfig{
		BaseURL:      strings.TrimRight(cmd.String("wfm-base-url"), "/"),
		DeviceID:     cmd.String("device-id"),
		PollInterval: cmd.Duration("poll-interval"),
	}
	verbose = cmd.Bool("verbose")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	st := &state{Deployments: map[string]deploymentCacheEntry{}}

	infof("client start deviceId=%s base=%s interval=%s", cfg.DeviceID, cfg.BaseURL, cfg.PollInterval)

	sigs := make(chan os.Signal, 2)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-sigs; cancel() }()

	for {
		if err := pollOnce(ctx, httpClient, cfg, st); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			warnf("poll error: %v", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(cfg.PollInterval):
		}
	}
}

func infof(format string, args ...any) {
	if verbose {
		log.Printf("ℹ️  "+format, args...)
	}
}

func warnf(format string, args ...any) {
	log.Printf("⚠️  "+format, args...)
}

func errorf(format string, args ...any) {
	log.Printf("❌ "+format, args...)
}

func successf(format string, args ...any) {
	if verbose {
		log.Printf("✅ "+format, args...)
	}
}

func actionf(action string, format string, args ...any) {
	log.Printf("→ %-8s %s", strings.ToUpper(action), fmt.Sprintf(format, args...))
}

func tracef(format string, args ...any) {
	if verbose {
		log.Printf("·  "+format, args...)
	}
}

// pollOnce fetches the manifest, performs diff, fetches new/updated deployments
func pollOnce(ctx context.Context, c *http.Client, cfg clientConfig, st *state) error {
	manifest, etag, err := fetchManifest(ctx, c, cfg, st.ManifestETag)
	if err != nil {
		return err
	}
	if manifest == nil { // no changes / server has returned 304 Not Modified
		return nil
	}

	// mitigate rollback attack
	if manifest.ManifestVersion <= st.ManifestVersion {
		warnf("ignoring non-increasing manifest version old=%d new=%d", st.ManifestVersion, manifest.ManifestVersion)
		return nil
	}

	st.ManifestVersion = manifest.ManifestVersion
	if etag == "" {
		warnf("manifest response missing ETag; server is non-compliant with spec, cache validator cleared")
	}
	st.ManifestETag = etag

	desiredIDs := make(map[string]struct{}, len(manifest.Deployments))
	resolved := make(map[string]resolvedDeployment, len(manifest.Deployments))

	// Initial sync: fetch bundle to accelerate first sync
	if !st.BundleFetched && len(st.Deployments) == 0 {
		entries, ok := fetchBundleOnce(ctx, c, cfg.BaseURL, manifest.Bundle, manifest.Deployments)
		if ok {
			for depID, entry := range entries {
				resolved[depID] = entry
			}
			st.BundleFetched = true
		}
	}

	// Continuous sync: fetch individual deployment descriptors whenever needed
	for _, d := range manifest.Deployments {
		desiredIDs[d.DeploymentId] = struct{}{}

		if !isSupportedDigest(d.DeploymentId, d.Digest) {
			continue
		}

		if entry, ok := resolved[d.DeploymentId]; ok {
			// descriptor resolved from initial sync / bundle
			entry.Digest = d.Digest
			resolved[d.DeploymentId] = entry
			continue
		}

		current, have := st.Deployments[d.DeploymentId]
		if have && current.Digest == d.Digest {
			// descriptor resolved from earlier sync / served from cache
			resolved[d.DeploymentId] = resolvedDeployment{Digest: d.Digest}
			continue
		}

		// new descriptor: resolve from server
		pd, err := fetchDeployment(ctx, c, resolveURL(cfg.BaseURL, d.URL), d.Digest)
		if err != nil {
			errorf("fetch deployment failed deploymentId=%s digest=%s err=%v", d.DeploymentId, d.Digest, err)
			continue
		}
		resolved[d.DeploymentId] = resolvedDeployment{Digest: d.Digest, Descriptor: &pd}
	}

	reconcileDeployments(st, desiredIDs, resolved)
	return nil
}

func isSupportedDigest(deploymentID, digest string) bool {
	if !digestRe.MatchString(digest) {
		warnf("skip invalid digest deploymentId=%s digest=%s", deploymentID, digest)
		return false
	}

	if !strings.HasPrefix(digest, "sha256:") {
		warnf("unsupported digest algorithm deploymentId=%s digest=%s", deploymentID, digest)
		return false
	}

	return true
}

func fetchManifest(ctx context.Context, c *http.Client, cfg clientConfig, etag string) (*common.GetDeploymentManifestResponse, string, error) {
	manifestURL := resolveURL(cfg.BaseURL, fmt.Sprintf("/api/v1/devices/%s/deployments", cfg.DeviceID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("manifest request build failed: %w", err)
	}
	if etag != "" {
		// send previous manifest ETag via If-None-Match to save some bandwidth
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("manifest request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		io.Copy(io.Discard, resp.Body)
		return nil, "", nil
	case http.StatusOK:
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("manifest read failed: %w", err)
		}
		var manifest common.GetDeploymentManifestResponse
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, "", fmt.Errorf("manifest parse error: %w", err)
		}
		return &manifest, resp.Header.Get("ETag"), nil
	default:
		io.Copy(io.Discard, resp.Body)
		return nil, "", fmt.Errorf("unexpected manifest status %d", resp.StatusCode)
	}
}

func fetchDeployment(ctx context.Context, c *http.Client, url, expectedDigest string) (common.ApplicationDeploymentDescriptor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return common.ApplicationDeploymentDescriptor{}, fmt.Errorf("deployment request build failed: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return common.ApplicationDeploymentDescriptor{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		return common.ApplicationDeploymentDescriptor{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return common.ApplicationDeploymentDescriptor{}, fmt.Errorf("deployment read failed: %w", err)
	}
	// validate that the fetched deployment matches the digest provided in the manifest
	if common.CalculateDigest(body) != expectedDigest {
		return common.ApplicationDeploymentDescriptor{}, fmt.Errorf("digest mismatch")
	}
	var desc common.ApplicationDeploymentDescriptor
	if err := yaml.Unmarshal(body, &desc); err != nil {
		return desc, err
	}
	return desc, nil
}

func fetchBundleOnce(ctx context.Context, c *http.Client, baseURL string, b *common.BundleDTO, deployments []common.DeploymentDTO) (map[string]resolvedDeployment, bool) {
	// bundle may be null when the server has no deployments assigned to this client
	if b == nil || b.URL == "" {
		return nil, true
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolveURL(baseURL, b.URL), nil)
	if err != nil {
		errorf("bundle request build failed: %v", err)
		return nil, false
	}
	resp, err := c.Do(req)
	if err != nil {
		errorf("bundle fetch error: %v", err)
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		warnf("bundle status=%d", resp.StatusCode)
		return nil, false
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		errorf("bundle read error: %v", err)
		return nil, false
	}
	if common.CalculateDigest(raw) != b.Digest {
		warnf("bundle digest mismatch expected=%s", b.Digest)
		return nil, false
	}
	entries, processed, ok := processBundleArchive(raw, deployments)
	if !ok {
		return nil, false
	}
	successf("bundle processed digest=%s filesProcessed=%d", b.Digest, processed)
	return entries, true
}

func processBundleArchive(raw []byte, deployments []common.DeploymentDTO) (map[string]resolvedDeployment, int, bool) {
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		errorf("bundle gzip error: %v", err)
		return nil, 0, false
	}
	defer zr.Close()
	tr := tar.NewReader(zr)

	// Build reverse lookup: digest -> deploymentId for quick association.
	digestToDep := make(map[string]string, len(deployments))
	for _, d := range deployments {
		digestToDep[d.Digest] = d.DeploymentId
	}

	entries := make(map[string]resolvedDeployment, len(digestToDep))
	processed := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			errorf("bundle tar read error: %v", err)
			return nil, processed, false
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		// Only consider .yaml or .yml
		if ext := strings.ToLower(filepath.Ext(hdr.Name)); ext != ".yaml" && ext != ".yml" {
			continue
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			errorf("bundle file read error: name=%s err=%v", hdr.Name, err)
			continue
		}
		dg := common.CalculateDigest(content)
		depID, ok := digestToDep[dg]
		if !ok {
			// Bundle entry / YAML file not referenced in the manifest
			warnf("bundle entry ignored name=%s digest=%s", hdr.Name, dg)
			continue
		}
		var desc common.ApplicationDeploymentDescriptor
		if err := yaml.Unmarshal(content, &desc); err != nil {
			errorf("bundle yaml parse error deploymentId=%s name=%s err=%v", depID, hdr.Name, err)
			continue
		}
		descCopy := desc
		entries[depID] = resolvedDeployment{Digest: dg, Descriptor: &descCopy}
		processed++
	}
	return entries, processed, true
}

func reconcileDeployments(st *state, desiredIDs map[string]struct{}, resolved map[string]resolvedDeployment) {
	// this map contains a plan that captures removals (nil entries) as well as updates and additions
	plan := make(map[string]*resolvedDeployment, len(st.Deployments)+len(resolved))

	for depID := range st.Deployments {
		if _, ok := desiredIDs[depID]; !ok {
			plan[depID] = nil
		}
	}

	for depID, entry := range resolved {
		entryCopy := entry
		plan[depID] = &entryCopy
	}

	for depID, desired := range plan {
		applyDeploymentChange(st, depID, desired)
	}
}

// resolveURL returns an absolute URL for a possibly relative reference.
func resolveURL(base, ref string) string {
	if ref == "" {
		return ref
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	// ensure base has scheme
	if !(strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://")) {
		return ref // will likely error upstream, surfacing misconfiguration
	}
	if strings.HasPrefix(ref, "/") {
		return strings.TrimRight(base, "/") + ref
	}
	return strings.TrimRight(base, "/") + "/" + ref
}

// applyDeploymentChange logs the action and mutates cached deployment state based on the desired descriptor
func applyDeploymentChange(st *state, deploymentID string, desired *resolvedDeployment) {
	existing, have := st.Deployments[deploymentID]

	if desired == nil {
		if !have {
			return
		}
		actionf("undeploy", "deploymentId=%s appId=%s name=%s digest=%s", deploymentID, existing.ApplicationID, existing.Name, existing.Digest)
		delete(st.Deployments, deploymentID)
		return
	}

	digest := desired.Digest
	desc := desired.Descriptor

	next := deploymentCacheEntry{Digest: digest}
	if desc != nil {
		next.ApplicationID = desc.Metadata.Annotations.ApplicationId
		next.Name = desc.Metadata.Name
	} else if have {
		next.ApplicationID = existing.ApplicationID
		next.Name = existing.Name
	}

	switch {
	case !have:
		actionf("deploy", "deploymentId=%s appId=%s name=%s digest=%s", deploymentID, next.ApplicationID, next.Name, digest)
		st.Deployments[deploymentID] = next
	case existing.Digest != digest:
		actionf("update", "deploymentId=%s appId=%s name=%s oldDigest=%s newDigest=%s", deploymentID, next.ApplicationID, next.Name, existing.Digest, digest)
		st.Deployments[deploymentID] = next
	default:
		tracef("noop deploymentId=%s appId=%s name=%s digest=%s", deploymentID, next.ApplicationID, next.Name, digest)
	}
}

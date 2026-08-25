//go:build e2e

package e2e_test

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func enrollFactoryCertMTLSForRenewTest(t *testing.T, agent, cert, key string) (svidCert, svidKey, stateFile string, leaf *x509.Certificate) {
	t.Helper()

	outDir := t.TempDir()
	svidCert = filepath.Join(outDir, "svid.pem")
	svidKey = filepath.Join(outDir, "svid.key")
	stateFile = filepath.Join(outDir, "state.json")

	enrollArgs := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"enroll", "factory-cert-mtls",
		"--factory-cert", filepath.Join("..", "..", cert),
		"--factory-key", filepath.Join("..", "..", key),
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	}
	if out, err := exec.Command(agent, enrollArgs...).CombinedOutput(); err != nil {
		t.Fatalf("enroll failed: %v\n%s", err, out)
	}

	leaf = readLeafCert(t, svidCert)
	assertStateOmitsPrivateKey(t, stateFile)
	return svidCert, svidKey, stateFile, leaf
}

func readLeafCert(t *testing.T, path string) *x509.Certificate {
	t.Helper()

	pemBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cert %s: %v", path, err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		t.Fatalf("decode cert %s: no PEM block", path)
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert %s: %v", path, err)
	}
	return leaf
}

func assertStateOmitsPrivateKey(t *testing.T, path string) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state %s: %v", path, err)
	}
	if strings.Contains(string(raw), "privateKeyPem") {
		t.Fatalf("state.json should not persist privateKeyPem:\n%s", raw)
	}
}

// TestDeviceAgent_RenewAfterFactoryCertMTLS enrolls a device then renews its
// SVID via the device-agent CLI. Asserts that:
//   - renewal returns 200 and a new leaf with the SAME SPIFFE ID,
//   - the renewed leaf's serial differs from the enrolled leaf's serial.
func TestDeviceAgent_RenewAfterFactoryCertMTLS(t *testing.T) {
	agent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("binary missing - run `make build` first: %v", err)
	}

	svidCert, svidKey, stateFile, originalLeaf := enrollFactoryCertMTLSForRenewTest(t, agent, factoryCert, factoryKey)

	renewArgs := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"renew",
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	}
	out, err := exec.Command(agent, renewArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("renew failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "status 200") {
		t.Errorf("expected 'status 200' in output, got:\n%s", out)
	}

	newLeaf := readLeafCert(t, svidCert)
	assertStateOmitsPrivateKey(t, stateFile)

	if newLeaf.URIs[0].String() != originalLeaf.URIs[0].String() {
		t.Errorf("SPIFFE ID changed across renewal: %v -> %v", originalLeaf.URIs, newLeaf.URIs)
	}
	if newLeaf.SerialNumber.Cmp(originalLeaf.SerialNumber) == 0 {
		t.Errorf("renewed leaf serial equals original; expected new serial")
	}
}

func TestDeviceAgent_RenewSameKeyAfterFactoryCertMTLS(t *testing.T) {
	agent := filepath.Join("..", "..", "bin", "device-agent")
	if _, err := os.Stat(agent); err != nil {
		t.Fatalf("binary missing - run `make build` first: %v", err)
	}

	svidCert, svidKey, stateFile, originalLeaf := enrollFactoryCertMTLSForRenewTest(t, agent, factoryCert2, factoryKey2)
	originalPublicKeyDER, err := x509.MarshalPKIXPublicKey(originalLeaf.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey(original): %v", err)
	}

	renewArgs := []string{
		"--mis-base-url", misBaseURL,
		"--mis-trust-anchor", filepath.Join("..", "..", misTrustAnchor),
		"renew",
		"--same-key",
		"--svid-cert", svidCert,
		"--svid-key", svidKey,
		"--state-file", stateFile,
	}
	out, err := exec.Command(agent, renewArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("same-key renew failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "status 200") {
		t.Errorf("expected 'status 200' in output, got:\n%s", out)
	}

	newLeaf := readLeafCert(t, svidCert)
	assertStateOmitsPrivateKey(t, stateFile)
	newPublicKeyDER, err := x509.MarshalPKIXPublicKey(newLeaf.PublicKey)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey(renewed): %v", err)
	}

	if newLeaf.URIs[0].String() != originalLeaf.URIs[0].String() {
		t.Errorf("SPIFFE ID changed across same-key renewal: %v -> %v", originalLeaf.URIs, newLeaf.URIs)
	}
	if newLeaf.SerialNumber.Cmp(originalLeaf.SerialNumber) == 0 {
		t.Errorf("same-key renewal leaf serial equals original; expected new serial")
	}
	if !bytes.Equal(originalPublicKeyDER, newPublicKeyDER) {
		t.Error("same-key renewal changed the leaf public key")
	}
}

// MIS Discovery & Trust commands: get-discovery and get-bundle.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
)

func getDiscovery(ctx context.Context, cmd *cli.Command) error {
	client, err := httpclient.New(cmd.Root().String("mis-trust-anchor"))
	if err != nil {
		return err
	}
	doc, err := discovery.Fetch(ctx, client, cmd.Root().String("mis-base-url"))
	if err != nil {
		return err
	}
	return jsonPrint(doc)
}

func getBundle(ctx context.Context, cmd *cli.Command) error {
	client, err := httpclient.New(cmd.Root().String("mis-trust-anchor"))
	if err != nil {
		return err
	}
	doc, err := discovery.Fetch(ctx, client, cmd.Root().String("mis-base-url"))
	if err != nil {
		return err
	}
	result, err := bundle.Fetch(ctx, client, doc.TrustBundleURI, cmd.String("if-modified-since"))
	if err != nil {
		return err
	}
	if result.NotModified {
		fmt.Println("304 Not Modified (cached copy is still valid)")
		return nil
	}
	fmt.Printf("Last-Modified: %s\n\n", result.LastModified)
	var raw any
	if err := json.Unmarshal(result.Body, &raw); err != nil {
		return errors.Join(errors.New("parse bundle map"), err)
	}
	return jsonPrint(raw)
}

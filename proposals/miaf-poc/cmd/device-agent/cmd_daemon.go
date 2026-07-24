// Daemon and ctl commands: run the long-lived daemon and send it control messages.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/margo/miaf-poc/pkg/device-agent/ctl"
	"github.com/margo/miaf-poc/pkg/device-agent/daemon"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
)

func runDaemon(ctx context.Context, cmd *cli.Command) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	stateFile, svidCert, svidKey := dataDirPaths(cmd)
	return daemon.Run(ctx, daemon.Config{
		StateFile:        stateFile,
		SVIDCertFile:     svidCert,
		SVIDKeyFile:      svidKey,
		MISBaseURL:       cmd.Root().String("mis-base-url"),
		MISTrustAnchor:   cmd.Root().String("mis-trust-anchor"),
		RenewAuth:        cmd.String("renew-auth"),
		SocketPath:       cmd.String("socket"),
		BundleEvery:      cmd.Duration("bundle-every"),
		RevocationsEvery: cmd.Duration("revocations-every"),
		RenewalRatio:     cmd.Float64("renewal-ratio"),
		JWTCacheTTL:      cmd.Duration("jwt-cache-ttl"),
		Logger:           logger,
	})
}

func ctlStatus(ctx context.Context, cmd *cli.Command) error {
	var data identitymanager.StatusReply
	if err := ctl.Call(ctx, cmd.Lineage()[1].String("socket"),
		ctl.Request{Cmd: "status"}, &data); err != nil {
		return err
	}
	return jsonPrint(data)
}

func ctlExchangeJWT(ctx context.Context, cmd *cli.Command) error {
	var data identitymanager.ExchangeJWTReply
	if err := ctl.Call(ctx, cmd.Lineage()[1].String("socket"),
		ctl.Request{
			Cmd:        "exchange-jwt",
			Audiences:  cmd.StringSlice("audience"),
			TTLSeconds: int(cmd.Duration("ttl") / time.Second),
		}, &data); err != nil {
		return err
	}
	return jsonPrint(data)
}

func ctlForceRenew(ctx context.Context, cmd *cli.Command) error {
	var data identitymanager.RenewReply
	if err := ctl.Call(ctx, cmd.Lineage()[1].String("socket"),
		ctl.Request{Cmd: "force-renew"}, &data); err != nil {
		return err
	}
	return jsonPrint(data)
}

func ctlForceBundle(ctx context.Context, cmd *cli.Command) error {
	var data identitymanager.RefreshBundleReply
	if err := ctl.Call(ctx, cmd.Lineage()[1].String("socket"),
		ctl.Request{Cmd: "force-bundle-refresh"}, &data); err != nil {
		return err
	}
	return jsonPrint(data)
}

func ctlForceRevocations(ctx context.Context, cmd *cli.Command) error {
	var data identitymanager.CheckRevocationsReply
	if err := ctl.Call(ctx, cmd.Lineage()[1].String("socket"),
		ctl.Request{Cmd: "force-revocations-check"}, &data); err != nil {
		return err
	}
	return jsonPrint(data)
}


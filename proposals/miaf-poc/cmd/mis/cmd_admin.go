// Admin subcommand actions: generate-token, generate-replacement-ticket, revoke.
package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/margo/miaf-poc/cmd/mis/internal/adminclient"
)

func runGenerateToken(ctx context.Context, cmd *cli.Command) error {
	adminURL := cmd.String("admin-url")
	ttl := cmd.Duration("ttl")
	if ttl == 0 {
		ttl = cmd.Duration("enrollment-token-default-ttl")
	}
	if ttl <= 0 {
		return fmt.Errorf("ttl must be positive, got %s", ttl)
	}
	resp, err := adminclient.New(adminURL).CreateEnrollmentToken(ctx, ttl)
	if err != nil {
		return fmt.Errorf("create enrollment token (%s): %w", adminURL, err)
	}
	fmt.Printf("token_id:   %s\n", resp.TokenID)
	fmt.Printf("secret:     %s\n", resp.Secret)
	fmt.Printf("expires_at: %s\n", resp.ExpiresAt)
	fmt.Println("Provision the secret on the device out-of-band.")
	return nil
}

func runGenerateReplacementTicket(ctx context.Context, cmd *cli.Command) error {
	adminURL := cmd.String("admin-url")
	spiffeID := cmd.String("spiffe-id")
	ttl := cmd.Duration("ttl")
	if spiffeID == "" {
		return fmt.Errorf("--spiffe-id is required")
	}
	if ttl <= 0 {
		return fmt.Errorf("--ttl must be positive, got %s", ttl)
	}
	resp, err := adminclient.New(adminURL).CreateReplacementTicket(ctx, spiffeID, ttl)
	if err != nil {
		return fmt.Errorf("create replacement ticket (%s): %w", adminURL, err)
	}
	fmt.Printf("ticket:     %s\n", resp.Ticket)
	fmt.Printf("spiffe_id:  %s\n", resp.SPIFFEID)
	fmt.Printf("expires_at: %s\n", resp.ExpiresAt)
	fmt.Println("Hand the ticket to the device operator out-of-band.")
	return nil
}

func runRevoke(ctx context.Context, cmd *cli.Command) error {
	adminURL := cmd.String("admin-url")
	spiffeID := cmd.String("spiffe-id")
	reason := cmd.String("reason")
	if spiffeID == "" {
		return fmt.Errorf("--spiffe-id is required")
	}
	resp, err := adminclient.New(adminURL).Revoke(ctx, spiffeID, reason)
	if err != nil {
		return fmt.Errorf("revoke (%s): %w", adminURL, err)
	}
	fmt.Printf("spiffe_id:       %s\n", resp.SPIFFEID)
	if len(resp.RevokedSerials) > 0 {
		fmt.Println("revoked_serials:")
		for _, serial := range resp.RevokedSerials {
			fmt.Printf("  - %s\n", serial)
		}
	}
	if len(resp.AlreadyRevoked) > 0 {
		fmt.Println("already_revoked:")
		for _, serial := range resp.AlreadyRevoked {
			fmt.Printf("  - %s\n", serial)
		}
	}
	return nil
}

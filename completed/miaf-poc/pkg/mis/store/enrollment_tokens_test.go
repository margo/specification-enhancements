package store_test

import (
	"context"
	"crypto/rand"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func openStore(t *testing.T) tokens.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "tokens.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	ts := s.EnrollmentTokens()
	return ts
}

func TestCreate_ReturnsUsableSecret(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, err := ts.Create(context.Background(), 5*time.Minute, now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.TokenID == "" {
		t.Error("TokenID empty")
	}
	if c.ExpiresAt.Before(now) {
		t.Errorf("ExpiresAt in past: %s", c.ExpiresAt)
	}
	// Round-trip the secret through the Store's Consume path.
	got, err := ts.Consume(context.Background(), c.Secret.Hash(), now)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if got != c.TokenID {
		t.Errorf("Consume token_id = %q, want %q", got, c.TokenID)
	}
}

func TestConsume_Unknown_ReturnsErrNotFound(t *testing.T) {
	ts := openStore(t)
	sec, _ := tokens.NewSecret(rand.Reader)
	_, err := ts.Consume(context.Background(), sec.Hash(), time.Now())
	if !errors.Is(err, tokens.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestConsume_Expired_ReturnsErrExpired(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, err := ts.Create(context.Background(), time.Second, now)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := ts.Consume(context.Background(), c.Secret.Hash(), now.Add(time.Minute)); !errors.Is(err, tokens.ErrExpired) {
		t.Errorf("err = %v, want ErrExpired", err)
	}
}

func TestConsume_Twice_ReturnsErrAlreadyUsed(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, _ := ts.Create(context.Background(), 5*time.Minute, now)
	if _, err := ts.Consume(context.Background(), c.Secret.Hash(), now); err != nil {
		t.Fatalf("first Consume: %v", err)
	}
	_, err := ts.Consume(context.Background(), c.Secret.Hash(), now)
	if !errors.Is(err, tokens.ErrAlreadyUsed) {
		t.Errorf("err = %v, want ErrAlreadyUsed", err)
	}
}

func TestConsume_Concurrent_OnlyOneWins(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, _ := ts.Create(context.Background(), 5*time.Minute, now)

	const N = 10
	results := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			_, err := ts.Consume(context.Background(), c.Secret.Hash(), now)
			results <- err
		}()
	}
	accepted := 0
	for i := 0; i < N; i++ {
		if err := <-results; err == nil {
			accepted++
		}
	}
	if accepted != 1 {
		t.Errorf("accepted = %d, want exactly 1", accepted)
	}
}

func TestRecordOutcomeAndFindConsumed_RoundTrip(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, _ := ts.Create(context.Background(), 5*time.Minute, now)
	tid, err := ts.Consume(context.Background(), c.Secret.Hash(), now)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}
	out := tokens.Outcome{
		TokenID:             tid,
		StatusCode:          201,
		CertificateChainPEM: []string{"-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n"},
		SVIDProfileURI:      "https://margo.org/profiles/spiffe/x509-svid/v1",
		CSRPubKeySHA256:     []byte{1, 2, 3, 4},
		LDIUUID:             "11111111-1111-1111-1111-111111111111",
		CompletedAt:         now,
		RetryWindowEnds:     now.Add(5 * time.Minute),
	}
	if err := ts.RecordOutcome(context.Background(), out); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}

	gotID, gotOut, err := ts.FindConsumed(context.Background(), c.Secret.Hash(), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("FindConsumed: %v", err)
	}
	if gotID != tid {
		t.Errorf("token_id = %q, want %q", gotID, tid)
	}
	if gotOut.StatusCode != 201 || gotOut.LDIUUID != out.LDIUUID {
		t.Errorf("outcome mismatch: %+v", gotOut)
	}
}

func TestFindConsumed_AfterRetryWindow_ErrNotFound(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, _ := ts.Create(context.Background(), 10*time.Minute, now)
	tid, _ := ts.Consume(context.Background(), c.Secret.Hash(), now)
	_ = ts.RecordOutcome(context.Background(), tokens.Outcome{
		TokenID:         tid,
		StatusCode:      201,
		CompletedAt:     now,
		RetryWindowEnds: now.Add(5 * time.Minute),
		SVIDProfileURI:  "https://margo.org/profiles/spiffe/x509-svid/v1",
		CSRPubKeySHA256: []byte{9},
	})
	_, _, err := ts.FindConsumed(context.Background(), c.Secret.Hash(), now.Add(6*time.Minute))
	if !errors.Is(err, tokens.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRecordOutcome_SecondCall_Fails(t *testing.T) {
	ts := openStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	c, _ := ts.Create(context.Background(), 5*time.Minute, now)
	tid, _ := ts.Consume(context.Background(), c.Secret.Hash(), now)
	out := tokens.Outcome{
		TokenID: tid, StatusCode: 201,
		CompletedAt: now, RetryWindowEnds: now.Add(5 * time.Minute),
		SVIDProfileURI:  "https://margo.org/profiles/spiffe/x509-svid/v1",
		CSRPubKeySHA256: []byte{0},
	}
	if err := ts.RecordOutcome(context.Background(), out); err != nil {
		t.Fatalf("first RecordOutcome: %v", err)
	}
	if err := ts.RecordOutcome(context.Background(), out); err == nil {
		t.Error("second RecordOutcome returned nil, want error (primary-key violation)")
	}
}

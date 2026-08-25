package httpclient_test

import (
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
)

func TestNew_MissingFile(t *testing.T) {
	if _, err := httpclient.New("does-not-exist.pem"); err == nil {
		t.Error("expected error on missing trust anchor file")
	}
}

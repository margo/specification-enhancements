package csr_test

import (
	"crypto/x509"
	"encoding/base64"
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/csr"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
)

func TestBuild_ES256_ParseableByStdlib(t *testing.T) {
	key, _ := keygen.Generate(keygen.ES256)
	encoded, err := csr.Build(key)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	parsed, err := x509.ParseCertificateRequest(der)
	if err != nil {
		t.Fatalf("ParseCertificateRequest: %v", err)
	}
	if err := parsed.CheckSignature(); err != nil {
		t.Fatalf("CheckSignature: %v", err)
	}
	if parsed.SignatureAlgorithm != x509.ECDSAWithSHA256 {
		t.Errorf("alg = %s, want ECDSAWithSHA256", parsed.SignatureAlgorithm)
	}
}

func TestBuild_PS256_ParseableByStdlib(t *testing.T) {
	key, _ := keygen.Generate(keygen.PS256)
	encoded, err := csr.Build(key)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	der, _ := base64.StdEncoding.DecodeString(encoded)
	parsed, err := x509.ParseCertificateRequest(der)
	if err != nil {
		t.Fatalf("ParseCertificateRequest: %v", err)
	}
	if err := parsed.CheckSignature(); err != nil {
		t.Fatalf("CheckSignature: %v", err)
	}
	if parsed.SignatureAlgorithm != x509.SHA256WithRSAPSS {
		t.Errorf("alg = %s, want SHA256WithRSAPSS", parsed.SignatureAlgorithm)
	}
}

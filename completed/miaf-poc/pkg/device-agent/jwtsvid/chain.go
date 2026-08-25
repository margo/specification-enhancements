package jwtsvid

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

// LoadSVIDChain reads a multi-certificate PEM file and returns the chain in
// the order it appears on disk. The leaf SVID MUST be the first PEM block.
// Returns an error if the file is empty or contains no CERTIFICATE blocks.
func LoadSVIDChain(path string) ([]*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var chain []*x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		chain = append(chain, c)
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("no CERTIFICATE blocks in %s", path)
	}
	return chain, nil
}

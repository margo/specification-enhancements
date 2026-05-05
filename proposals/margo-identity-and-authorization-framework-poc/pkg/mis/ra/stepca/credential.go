package stepca

import (
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/go-jose/go-jose/v4"
)

// CredentialConfig parametrises LoadCredential.
type CredentialConfig struct {
	// ProvisionerFile is the JSON file exported from Step CA's ca.json by the
	// custom step-ca entrypoint. It contains:
	//   - name:         provisioner name
	//   - type:         "JWK"
	//   - key:          the provisioner's public JWK
	//   - encryptedKey: JWE-wrapped private JWK (compact serialization)
	ProvisionerFile string

	// PasswordFile is a text file whose contents unlock the JWE in
	// ProvisionerFile. Trailing newlines are stripped.
	PasswordFile string
}

// Credential is a loaded, decrypted JWK provisioner credential ready to mint
// one-time tokens (OTTs) for Step CA's /1.0/sign endpoint.
type Credential struct {
	ProvisionerName string
	KeyID           string        // `kid` from the public JWK (used as JWT header)
	SigningKey      crypto.Signer // the decrypted private key
	PublicJWK       jose.JSONWebKey
}

type provisionerFile struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Key          json.RawMessage `json:"key"`
	EncryptedKey string          `json:"encryptedKey"`
}

// LoadCredential reads ProvisionerFile + PasswordFile, decrypts the private
// JWK, and returns a ready-to-use Credential.
func LoadCredential(cfg CredentialConfig) (*Credential, error) {
	if cfg.ProvisionerFile == "" || cfg.PasswordFile == "" {
		return nil, errors.New("stepca: ProvisionerFile and PasswordFile are required")
	}
	raw, err := os.ReadFile(cfg.ProvisionerFile)
	if err != nil {
		return nil, fmt.Errorf("stepca: read provisioner file: %w", err)
	}
	var pf provisionerFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		return nil, fmt.Errorf("stepca: parse provisioner file: %w", err)
	}
	if !strings.EqualFold(pf.Type, "JWK") {
		return nil, fmt.Errorf("stepca: provisioner type = %q, only JWK is supported", pf.Type)
	}
	if pf.EncryptedKey == "" {
		return nil, errors.New("stepca: provisioner file missing encryptedKey")
	}
	pwdRaw, err := os.ReadFile(cfg.PasswordFile)
	if err != nil {
		return nil, fmt.Errorf("stepca: read password file: %w", err)
	}
	password := strings.TrimRight(string(pwdRaw), "\r\n")

	var pubJWK jose.JSONWebKey
	if err := json.Unmarshal(pf.Key, &pubJWK); err != nil {
		return nil, fmt.Errorf("stepca: parse public JWK: %w", err)
	}

	jwe, err := jose.ParseEncrypted(pf.EncryptedKey,
		[]jose.KeyAlgorithm{jose.PBES2_HS256_A128KW, jose.PBES2_HS384_A192KW, jose.PBES2_HS512_A256KW},
		[]jose.ContentEncryption{jose.A128GCM, jose.A192GCM, jose.A256GCM, jose.A128CBC_HS256, jose.A192CBC_HS384, jose.A256CBC_HS512})
	if err != nil {
		return nil, fmt.Errorf("stepca: parse encryptedKey JWE: %w", err)
	}
	privBytes, err := jwe.Decrypt([]byte(password))
	if err != nil {
		return nil, fmt.Errorf("stepca: decrypt private JWK (wrong password?): %w", err)
	}
	var privJWK jose.JSONWebKey
	if err := json.Unmarshal(privBytes, &privJWK); err != nil {
		return nil, fmt.Errorf("stepca: parse private JWK: %w", err)
	}
	signer, ok := privJWK.Key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("stepca: private JWK key type %T is not a crypto.Signer", privJWK.Key)
	}
	return &Credential{
		ProvisionerName: pf.Name,
		KeyID:           pubJWK.KeyID,
		SigningKey:      signer,
		PublicJWK:       pubJWK,
	}, nil
}

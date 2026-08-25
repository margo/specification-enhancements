package stepca_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-jose/go-jose/v4"

	"github.com/margo/miaf-poc/pkg/mis/ra/stepca"
)

func b64url(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func paddedBytes(i *big.Int, sz int) []byte {
	b := i.Bytes()
	if len(b) >= sz {
		return b
	}
	out := make([]byte, sz)
	copy(out[sz-len(b):], b)
	return out
}

func generateES256KeyPair(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, &priv.PublicKey
}

func publicJWK(t *testing.T, pub *ecdsa.PublicKey) map[string]any {
	t.Helper()
	return map[string]any{
		"kty": "EC",
		"crv": "P-256",
		"x":   b64url(paddedBytes(pub.X, 32)),
		"y":   b64url(paddedBytes(pub.Y, 32)),
		"kid": "test-kid",
	}
}

func privateJWK(t *testing.T, priv *ecdsa.PrivateKey) map[string]any {
	t.Helper()
	jwk := publicJWK(t, &priv.PublicKey)
	jwk["d"] = b64url(paddedBytes(priv.D, 32))
	return jwk
}

// writeProvisionerFixture generates a fresh ES256 JWK, encrypts the private
// half with PBES2-HS256+A128KW under a test password, and writes the same
// JSON shape step-ca's entrypoint exports from ca.json.
func writeProvisionerFixture(t *testing.T, password string) (provisionerPath, passwordPath string) {
	t.Helper()
	dir := t.TempDir()

	priv, pub := generateES256KeyPair(t)
	privBytes, err := json.Marshal(privateJWK(t, priv))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := jose.NewEncrypter(
		jose.A128CBC_HS256, // step-ca's default content-encryption for JWK provisioners
		jose.Recipient{
			Algorithm:  jose.PBES2_HS256_A128KW,
			Key:        []byte(password),
			PBES2Count: 1000,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	jwe, err := enc.Encrypt(privBytes)
	if err != nil {
		t.Fatal(err)
	}
	encryptedKey, err := jwe.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}

	prov := map[string]any{
		"type":         "JWK",
		"name":         "margo-mis",
		"key":          publicJWK(t, pub),
		"encryptedKey": encryptedKey,
	}
	provBytes, _ := json.MarshalIndent(prov, "", "  ")
	provisionerPath = filepath.Join(dir, "provisioner.json")
	if err := os.WriteFile(provisionerPath, provBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	passwordPath = filepath.Join(dir, "password")
	if err := os.WriteFile(passwordPath, []byte(password+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return
}

func TestLoadCredential_HappyPath(t *testing.T) {
	provPath, pwPath := writeProvisionerFixture(t, "test-pw")
	cred, err := stepca.LoadCredential(stepca.CredentialConfig{
		ProvisionerFile: provPath,
		PasswordFile:    pwPath,
	})
	if err != nil {
		t.Fatalf("LoadCredential: %v", err)
	}
	if cred.ProvisionerName != "margo-mis" {
		t.Errorf("ProvisionerName = %q, want margo-mis", cred.ProvisionerName)
	}
	if cred.SigningKey == nil {
		t.Fatal("SigningKey is nil")
	}
	if _, ok := cred.SigningKey.(*ecdsa.PrivateKey); !ok {
		t.Errorf("SigningKey type = %T, want *ecdsa.PrivateKey", cred.SigningKey)
	}
	if cred.KeyID == "" {
		t.Error("KeyID is empty")
	}
}

func TestLoadCredential_WrongPassword(t *testing.T) {
	provPath, _ := writeProvisionerFixture(t, "correct-pw")
	dir := t.TempDir()
	wrongPath := filepath.Join(dir, "password")
	if err := os.WriteFile(wrongPath, []byte("wrong-pw"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := stepca.LoadCredential(stepca.CredentialConfig{
		ProvisionerFile: provPath,
		PasswordFile:    wrongPath,
	})
	if err == nil {
		t.Fatal("expected error on wrong password")
	}
}

func TestLoadCredential_MissingFiles(t *testing.T) {
	t.Run("missing provisioner file", func(t *testing.T) {
		_, err := stepca.LoadCredential(stepca.CredentialConfig{
			ProvisionerFile: "/no/such/provisioner.json",
			PasswordFile:    "/no/such/password",
		})
		if err == nil {
			t.Fatal("expected error for missing provisioner file")
		}
	})
	t.Run("missing password file", func(t *testing.T) {
		provPath, _ := writeProvisionerFixture(t, "pw")
		_, err := stepca.LoadCredential(stepca.CredentialConfig{
			ProvisionerFile: provPath,
			PasswordFile:    "/no/such/password",
		})
		if err == nil {
			t.Fatal("expected error for missing password file")
		}
	})
}

func TestLoadCredential_EmptyConfig(t *testing.T) {
	_, err := stepca.LoadCredential(stepca.CredentialConfig{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

package modules

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// signForTest produces a .sha256 digest (hex + "\n") and its ed25519
// signature for the given binary, using a throwaway key.
func signForTest(t *testing.T, binary []byte) (pub ed25519.PublicKey, digest, sig []byte) {
	t.Helper()
	p, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(binary)
	digest = []byte(hex.EncodeToString(sum[:]) + "\n")
	sig = ed25519.Sign(priv, digest)
	return p, digest, sig
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := t.TempDir() + "/" + name
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyDigestWithKeyAcceptsValidSignature(t *testing.T) {
	binary := []byte("fake netgrip binary")
	pub, digest, sig := signForTest(t, binary)
	path := writeTemp(t, "bin", binary)

	if err := verifyDigestWithKey(pub, path, digest, sig); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyDigestWithKeyRejectsBadSignature(t *testing.T) {
	binary := []byte("fake netgrip binary")
	pub, digest, _ := signForTest(t, binary)
	path := writeTemp(t, "bin", binary)

	badSig := make([]byte, ed25519.SignatureSize)
	if err := verifyDigestWithKey(pub, path, digest, badSig); err == nil {
		t.Fatal("expected a zeroed signature to be rejected")
	}
}

func TestVerifyDigestWithKeyRejectsWrongKey(t *testing.T) {
	binary := []byte("fake netgrip binary")
	_, digest, sig := signForTest(t, binary)
	otherPub, _, _ := signForTest(t, []byte("other"))
	path := writeTemp(t, "bin", binary)

	if err := verifyDigestWithKey(otherPub, path, digest, sig); err == nil {
		t.Fatal("expected a signature from another key to be rejected")
	}
}

func TestVerifyDigestWithKeyRejectsHashMismatch(t *testing.T) {
	binary := []byte("fake netgrip binary")
	pub, digest, sig := signForTest(t, binary)
	// Sign a digest that does not match the file actually written.
	path := writeTemp(t, "bin", []byte("a different binary"))

	if err := verifyDigestWithKey(pub, path, digest, sig); err == nil {
		t.Fatal("expected a hash mismatch to be rejected")
	}
}

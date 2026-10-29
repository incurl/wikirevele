package pocketbase

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCipherCreatesTheKeyWithTightPermissions verifies that the
// encryption key is not readable by anyone else.
func TestCipherCreatesTheKeyWithTightPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the file modes are not meaningful on Windows")
	}

	dir := t.TempDir()

	if _, err := NewCipher(dir); err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, keyFileName))
	if err != nil {
		t.Fatalf("stat the key: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("the key file has mode %o, want 600", mode)
	}
}

// TestCipherSecuresAnExistingKey verifies that a key file that was
// left with looser permissions is tightened.
//
// This is the regression test for the finding of the 2026-10-28 audit:
// os.WriteFile does not change the mode of an existing file, so a key
// that was copied or restored with 0644 stayed readable.
func TestCipherSecuresAnExistingKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the file modes are not meaningful on Windows")
	}

	dir := t.TempDir()
	keyPath := filepath.Join(dir, keyFileName)

	// A key file that is world readable.
	if err := os.WriteFile(keyPath, make([]byte, 32), 0o644); err != nil {
		t.Fatalf("write the key: %v", err)
	}
	if err := os.Chmod(keyPath, 0o644); err != nil {
		t.Fatalf("chmod the key: %v", err)
	}

	if _, err := NewCipher(dir); err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat the key: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0o600 {
		t.Errorf("the key file has mode %o, want it tightened to 600", mode)
	}
}

// TestCipherRoundTrip verifies that an encrypted value can be read
// back.
func TestCipherRoundTrip(t *testing.T) {
	cipher, err := NewCipher(t.TempDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	envelope, err := cipher.Encrypt([]byte("a secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if envelope == "a secret" {
		t.Fatal("the envelope is the plaintext")
	}

	plaintext, err := cipher.Decrypt(envelope)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plaintext) != "a secret" {
		t.Errorf("plaintext = %q, want %q", plaintext, "a secret")
	}
}

// TestCipherUsesAFreshNonce verifies that two encryptions of the same
// plaintext give different envelopes.
//
// A reused nonce in GCM is catastrophic: it leaks the XOR of the
// plaintexts and the authentication key.
func TestCipherUsesAFreshNonce(t *testing.T) {
	cipher, err := NewCipher(t.TempDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	first, err := cipher.Encrypt([]byte("the same secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	second, err := cipher.Encrypt([]byte("the same secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if first == second {
		t.Error("two encryptions of the same plaintext gave the same envelope: the nonce is reused")
	}
}

// TestCipherRejectsATamperedEnvelope verifies that the authentication
// works.
func TestCipherRejectsATamperedEnvelope(t *testing.T) {
	cipher, err := NewCipher(t.TempDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	envelope, err := cipher.Encrypt([]byte("a secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Flip a character in the middle of the envelope.
	tampered := []byte(envelope)
	tampered[len(tampered)/2] ^= 'A'

	if _, err := cipher.Decrypt(string(tampered)); err == nil {
		t.Error("a tampered envelope was accepted")
	}
}

// TestCipherRejectsAWrongKey verifies that another key cannot read the
// envelope.
func TestCipherRejectsAWrongKey(t *testing.T) {
	first, err := NewCipher(t.TempDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}
	second, err := NewCipher(t.TempDir())
	if err != nil {
		t.Fatalf("create cipher: %v", err)
	}

	envelope, err := first.Encrypt([]byte("a secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if _, err := second.Decrypt(envelope); err == nil {
		t.Error("another key decrypted the envelope")
	}
}

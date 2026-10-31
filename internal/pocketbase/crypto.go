package pocketbase

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Cipher is an AES-256-GCM encryptor used to protect the application
// secrets before they are written to the app_settings collection.
//
// PocketBase provides no generic secret-encryption facility, so
// Wikirevele stores an encrypted envelope in the record. The key never
// leaves the machine: it is generated on first run and kept in a 0600
// file inside the data directory.
type Cipher struct {
	aead cipher.AEAD
	mu   sync.Mutex
	key  []byte
}

// keyFileName is the name of the file holding the local encryption key.
const keyFileName = ".wikirevele_key"

// legacyKeyFileName is the name the key file had before the
// application was renamed.
//
// A key file that is still there is used as it is: the secrets of an
// existing installation must stay readable, and a new key would make
// every one of them unreadable.
const legacyKeyFileName = ".valkyrie_key"

// NewCipher loads the encryption key from dataDir, creating it on first
// use.
func NewCipher(dataDir string) (*Cipher, error) {
	keyPath := filepath.Join(dataDir, keyFileName)

	key, err := os.ReadFile(keyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("read encryption key: %w", err)
		}

		// The key file of the old name is used when it is the only
		// one there.
		legacy, legacyErr := os.ReadFile(filepath.Join(dataDir, legacyKeyFileName))
		if legacyErr == nil {
			key = legacy
		} else {
			key = make([]byte, 32)
			if _, err := rand.Read(key); err != nil {
				return nil, fmt.Errorf("generate encryption key: %w", err)
			}
		}

		if err := os.WriteFile(keyPath, key, 0o600); err != nil {
			return nil, fmt.Errorf("write encryption key: %w", err)
		}
	} else {
		// The mode of an existing file is left alone by WriteFile, and
		// the file may have been copied or restored with looser
		// permissions. The key must not be readable by anyone else.
		if err := os.Chmod(keyPath, 0o600); err != nil {
			return nil, fmt.Errorf("secure the encryption key: %w", err)
		}
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	return &Cipher{aead: aead, key: key}, nil
}

// Encrypt encrypts plaintext and returns a self-describing envelope
// encoded as base64.
func (c *Cipher) Encrypt(plaintext []byte) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := c.aead.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts an envelope produced by Encrypt.
func (c *Cipher) Decrypt(envelope string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	raw, err := base64.StdEncoding.DecodeString(envelope)
	if err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}

	nonceSize := c.aead.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("envelope is too short")
	}

	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// EncryptJSON marshals v and encrypts the result.
func (c *Cipher) EncryptJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal value: %w", err)
	}
	return c.Encrypt(raw)
}

// DecryptJSON decrypts an envelope and unmarshals it into v.
func (c *Cipher) DecryptJSON(envelope string, v any) error {
	raw, err := c.Decrypt(envelope)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("unmarshal value: %w", err)
	}
	return nil
}

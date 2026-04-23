package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"sync"
)

type KeyManager interface {
	KeyID() string
	WrapKey(ctx context.Context, plaintext []byte) (ciphertext string, nonce string, err error)
	UnwrapKey(ctx context.Context, ciphertext string, nonce string) ([]byte, error)
}

type EnvKMS struct {
	keyID     string
	masterKey []byte
}

var (
	defaultKMS     KeyManager
	defaultKMSOnce sync.Once
)

func DefaultKMS() KeyManager {
	defaultKMSOnce.Do(func() {
		defaultKMS = NewEnvKMS()
	})
	return defaultKMS
}

func NewEnvKMS() *EnvKMS {
	keyID := strings.TrimSpace(os.Getenv("KMS_KEY_ID"))
	if keyID == "" {
		keyID = "env-kms-v1"
	}

	key := strings.TrimSpace(os.Getenv("KMS_MASTER_KEY_B64"))
	if key == "" {
		fallback := os.Getenv("SESSION_COOKIE_SECRET")
		sum := sha256.Sum256([]byte(fallback))
		return &EnvKMS{
			keyID:     keyID,
			masterKey: sum[:],
		}
	}

	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 {
		sum := sha256.Sum256([]byte(key))
		raw = sum[:]
	}

	return &EnvKMS{
		keyID:     keyID,
		masterKey: raw,
	}
}

func (k *EnvKMS) KeyID() string {
	return k.keyID
}

func (k *EnvKMS) WrapKey(ctx context.Context, plaintext []byte) (string, string, error) {
	return encryptWithKey(k.masterKey, plaintext, []byte(k.keyID))
}

func (k *EnvKMS) UnwrapKey(ctx context.Context, ciphertext string, nonce string) ([]byte, error) {
	return decryptWithKey(k.masterKey, ciphertext, nonce, []byte(k.keyID))
}

func ResetForTests() {
	defaultKMS = nil
	defaultProtector = nil
	defaultKMSOnce = sync.Once{}
	defaultProtectorOnce = sync.Once{}
}

func encryptWithKey(key []byte, plaintext []byte, additionalData []byte) (string, string, error) {
	if len(key) != 32 {
		return "", "", fmt.Errorf("expected 32-byte key, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, additionalData)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

func decryptWithKey(key []byte, ciphertext string, nonce string, additionalData []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("expected 32-byte key, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	rawCiphertext, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	rawNonce, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(nil, rawNonce, rawCiphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("decrypt payload: %w", err)
	}
	return plaintext, nil
}

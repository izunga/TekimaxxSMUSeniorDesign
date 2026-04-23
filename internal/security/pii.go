package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
)

type EncryptedValue struct {
	Version        int    `json:"version"`
	Algorithm      string `json:"algorithm"`
	KeyID          string `json:"key_id"`
	LookupHash     string `json:"lookup_hash"`
	WrappedKey     string `json:"wrapped_key"`
	WrappedKeyNonce string `json:"wrapped_key_nonce"`
	DataNonce      string `json:"data_nonce"`
	Ciphertext     string `json:"ciphertext"`
}

type PIIProtector struct {
	kms KeyManager
}

var (
	defaultProtector     *PIIProtector
	defaultProtectorOnce sync.Once
)

func DefaultPIIProtector() *PIIProtector {
	defaultProtectorOnce.Do(func() {
		defaultProtector = &PIIProtector{kms: DefaultKMS()}
	})
	return defaultProtector
}

func (p *PIIProtector) SealEmail(ctx context.Context, email string) (string, string, string, error) {
	normalized := strings.TrimSpace(strings.ToLower(email))
	lookupHash := HashEmail(normalized)
	dataKey := make([]byte, 32)
	if _, err := rand.Read(dataKey); err != nil {
		return "", "", "", err
	}

	ciphertext, dataNonce, err := encryptWithKey(dataKey, []byte(email), []byte(lookupHash))
	if err != nil {
		return "", "", "", err
	}
	wrappedKey, wrappedNonce, err := p.kms.WrapKey(ctx, dataKey)
	if err != nil {
		return "", "", "", err
	}

	payload := EncryptedValue{
		Version:         1,
		Algorithm:       "AES-256-GCM",
		KeyID:           p.kms.KeyID(),
		LookupHash:      lookupHash,
		WrappedKey:      wrappedKey,
		WrappedKeyNonce: wrappedNonce,
		DataNonce:       dataNonce,
		Ciphertext:      ciphertext,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", "", err
	}

	return base64.StdEncoding.EncodeToString(raw), lookupHash, payload.KeyID, nil
}

func (p *PIIProtector) OpenEmail(ctx context.Context, sealed string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}

	var payload EncryptedValue
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}

	dataKey, err := p.kms.UnwrapKey(ctx, payload.WrappedKey, payload.WrappedKeyNonce)
	if err != nil {
		return "", err
	}
	plaintext, err := decryptWithKey(dataKey, payload.Ciphertext, payload.DataNonce, []byte(payload.LookupHash))
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func HashEmail(email string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(email))))
	return hex.EncodeToString(sum[:])
}

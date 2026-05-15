package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Service encrypts and decrypts small blobs (e.g. settings values) with AES-GCM.
type Service struct {
	key []byte
}

// NewService builds a Service from APP_MASTER_KEY. Empty key returns (nil, nil).
// Accepts: 64-char hex (32 bytes), base64 / raw base64 (32 decoded bytes), 32-byte UTF-8 string,
// or any other string (hashed with SHA-256 to 32 bytes).
func NewService(masterKey string) (*Service, error) {
	s := strings.TrimSpace(masterKey)
	if s == "" {
		return nil, nil
	}
	k, err := parseMasterKey(s)
	if err != nil {
		return nil, err
	}
	return &Service{key: k}, nil
}

func parseMasterKey(s string) ([]byte, error) {
	if len(s) == 64 {
		if b, err := hex.DecodeString(s); err == nil && len(b) == 32 {
			return b, nil
		}
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil && len(b) == 32 {
		return b, nil
	}
	if len(s) == 32 {
		return []byte(s), nil
	}
	sum := sha256.Sum256([]byte(s))
	return sum[:], nil
}

// Encrypt returns base64(nonce || ciphertext), nonce length = 12 (GCM standard).
func (s *Service) Encrypt(plaintext []byte) (string, error) {
	if s == nil || len(s.key) == 0 {
		return "", fmt.Errorf("encrypt: no master key configured")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", fmt.Errorf("encrypt: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encrypt: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encrypt: nonce: %w", err)
	}
	out := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawStdEncoding.EncodeToString(out), nil
}

// Decrypt decodes base64(nonce || ciphertext).
func (s *Service) Decrypt(encoded string) ([]byte, error) {
	if s == nil || len(s.key) == 0 {
		return nil, fmt.Errorf("decrypt: no master key configured")
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		// tolerate standard base64 with padding
		raw, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decrypt: base64: %w", err)
		}
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("decrypt: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("decrypt: gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, errors.New("decrypt: ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// MaskSecret returns a short display form for secrets (never logs or returns raw in lists when encrypted).
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 4 {
		return "****"
	}
	if len(r) <= 8 {
		return "****" + string(r[len(r)-2:])
	}
	return string(r[:3]) + "****" + string(r[len(r)-4:])
}

// LooksMasked indicates the client sent a masked placeholder; treat as "do not change secret".
func LooksMasked(s string) bool {
	return strings.Contains(s, "****")
}

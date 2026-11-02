package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Cipher 提供字段级加密与不可逆哈希（手机号）。
type Cipher struct {
	aead    cipher.AEAD
	hashKey []byte
}

// New 从 base64 的 32 字节密钥与哈希密钥构造。
func New(keyBase64, hashKey string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	if hashKey == "" {
		return nil, errors.New("phone hash key is required")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Cipher{aead: aead, hashKey: []byte(hashKey)}, nil
}

// Encrypt 返回 base64url(nonce || ciphertext)。
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	sealed := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Decrypt 解密 Encrypt 的输出。
func (c *Cipher) Decrypt(encoded string) (string, error) {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceSize := c.aead.NonceSize()
	if len(payload) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, sealed := payload[:nonceSize], payload[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

// Hash 返回 HMAC-SHA256 十六进制，用于唯一索引与查询。
func (c *Cipher) Hash(value string) string {
	mac := hmac.New(sha256.New, c.hashKey)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

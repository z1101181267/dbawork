// Package crypto 提供数据源凭据的 AES-GCM 加解密。
//
// 设计：主密钥为 32 字节裸密钥，落盘于 data/secret.key（首次启动自动生成），
// 密文与 nonce 分列存储（对应数据库中的 *_enc / *_iv 列）。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// KeySize AES-256 密钥长度。
const KeySize = 32

// LoadOrCreateKey 读取密钥文件；不存在时生成并落盘。
func LoadOrCreateKey(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("密钥路径为空")
	}
	b, err := os.ReadFile(path)
	if err == nil {
		if len(b) != KeySize {
			return nil, fmt.Errorf("密钥文件 %s 长度应为 %d 字节，实际 %d 字节", path, KeySize, len(b))
		}
		return b, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// Cipher AES-GCM 加解密器。
type Cipher struct {
	aead cipher.AEAD
}

// New 基于 32 字节密钥构建加解密器。
func New(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("密钥长度应为 %d 字节", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 加密明文；空明文返回 (nil, nil, nil)。
func (c *Cipher) Encrypt(plain []byte) (ct, nonce []byte, err error) {
	if len(plain) == 0 {
		return nil, nil, nil
	}
	nonce = make([]byte, c.aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ct = c.aead.Seal(nil, nonce, plain, nil)
	return ct, nonce, nil
}

// Decrypt 解密密文；空密文返回空串。
func (c *Cipher) Decrypt(ct, nonce []byte) ([]byte, error) {
	if len(ct) == 0 {
		return []byte{}, nil
	}
	if len(nonce) != c.aead.NonceSize() {
		return nil, errors.New("nonce 长度不合法")
	}
	return c.aead.Open(nil, nonce, ct, nil)
}

// EncryptString 加密字符串。
func (c *Cipher) EncryptString(s string) ([]byte, []byte, error) {
	return c.Encrypt([]byte(s))
}

// DecryptString 解密为字符串。
func (c *Cipher) DecryptString(ct, nonce []byte) (string, error) {
	b, err := c.Decrypt(ct, nonce)
	return string(b), err
}

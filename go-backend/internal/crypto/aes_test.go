package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func testKey() []byte {
	k := make([]byte, KeySize)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func TestRoundtrip(t *testing.T) {
	c, err := New(testKey())
	if err != nil {
		t.Fatal(err)
	}
	plain := "p@ssword-中文-🤖"
	ct, nonce, err := c.EncryptString(plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct, []byte(plain)) {
		t.Fatal("密文不应等于明文")
	}
	got, err := c.DecryptString(ct, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("got %q want %q", got, plain)
	}
}

func TestEmptyValue(t *testing.T) {
	c, _ := New(testKey())
	ct, n, err := c.EncryptString("")
	if err != nil || ct != nil || n != nil {
		t.Fatal("空明文应返回 nil")
	}
	s, err := c.DecryptString(nil, nil)
	if err != nil || s != "" {
		t.Fatal("空密文应解密为空串")
	}
}

func TestTamper(t *testing.T) {
	c, _ := New(testKey())
	ct, nonce, _ := c.EncryptString("secret")
	ct[0] ^= 0xFF
	if _, err := c.DecryptString(ct, nonce); err == nil {
		t.Fatal("篡改密文应解密失败")
	}
}

func TestLoadOrCreateKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	k1, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(k1) != KeySize {
		t.Fatal("密钥长度错误")
	}
	k2, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(k1, k2) {
		t.Fatal("重复加载密钥应一致")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("密钥文件应存在")
	}
}

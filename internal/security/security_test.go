package security

import (
	"bytes"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestSignVerify(t *testing.T) {
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"type":"order.created"}`)
	sig := Sign("secret", ts, body)
	if !Verify("secret", ts, sig, body, now) {
		t.Fatal("valid signature rejected")
	}
	if Verify("wrong", ts, sig, body, now) || Verify("secret", ts, sig, []byte("changed"), now) || Verify("secret", ts, sig, body, now.Add(6*time.Minute)) {
		t.Fatal("invalid signature accepted")
	}
}
func TestEncrypt(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	a, e := Encrypt(key, "sensitive")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := Encrypt(key, "sensitive")
	if a == b {
		t.Fatal("nonce reused")
	}
	plain, e := Decrypt(key, a)
	if e != nil || plain != "sensitive" {
		t.Fatal(e)
	}
	if _, e = Decrypt(bytes.Repeat([]byte{8}, 32), a); e == nil {
		t.Fatal("wrong key accepted")
	}
	if _, e = Decrypt(key, "AA"); e == nil {
		t.Fatal("short ciphertext accepted")
	}
}
func TestPassword(t *testing.T) {
	if _, e := Password("short"); e == nil {
		t.Fatal("short password")
	}
	h, e := Password("test-password-123")
	if e != nil || !CheckPassword(h, "test-password-123") || CheckPassword(h, "wrong") {
		t.Fatal("password verification")
	}
}
func TestDestination(t *testing.T) {
	for _, v := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "::ffff:127.0.0.1", "100.100.100.200"} {
		if PublicIP(net.ParseIP(v)) {
			t.Errorf("allowed %s", v)
		}
	}
	if !PublicIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public blocked")
	}
	if ValidateURL("http://localhost", false) == nil || ValidateURL("https://user:pass@host", false) == nil {
		t.Fatal("unsafe URL")
	}
}

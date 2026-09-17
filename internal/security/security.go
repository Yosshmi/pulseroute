package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"strings"
	"time"
)

func Token(prefix string) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("operating system entropy unavailable")
	}
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func Password(s string) (string, error) {
	if len(s) < 12 || len(s) > 72 {
		return "", fmt.Errorf("password must contain 12..72 bytes")
	}
	b, e := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	return string(b), e
}
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
func Encrypt(key []byte, s string) (string, error) {
	block, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", e
	}
	return base64.RawStdEncoding.EncodeToString(g.Seal(nonce, nonce, []byte(s), []byte("pulseroute:endpoint:v1"))), nil
}
func Decrypt(key []byte, s string) (string, error) {
	b, e := base64.RawStdEncoding.DecodeString(s)
	if e != nil {
		return "", e
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return "", e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return "", e
	}
	if len(b) < g.NonceSize() {
		return "", fmt.Errorf("invalid ciphertext")
	}
	p, e := g.Open(nil, b[:g.NonceSize()], b[g.NonceSize():], []byte("pulseroute:endpoint:v1"))
	return string(p), e
}
func Sign(secret, timestamp string, body []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(timestamp + "."))
	h.Write(body)
	return "v1=" + hex.EncodeToString(h.Sum(nil))
}
func Verify(secret, timestamp, signature string, body []byte, now time.Time) bool {
	seconds, e := strconv.ParseInt(timestamp, 10, 64)
	if e != nil {
		return false
	}
	t := time.Unix(seconds, 0)
	if t.Before(now.Add(-5*time.Minute)) || t.After(now.Add(5*time.Minute)) {
		return false
	}
	given, e := hex.DecodeString(strings.TrimPrefix(signature, "v1="))
	if e != nil || !strings.HasPrefix(signature, "v1=") {
		return false
	}
	expected, _ := hex.DecodeString(strings.TrimPrefix(Sign(secret, timestamp, body), "v1="))
	return hmac.Equal(given, expected)
}

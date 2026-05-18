package integration

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type SignedHeaders struct {
	RequestId string
	Timestamp string
	Nonce     string
	Signature string
	BodyHash  string
}

func NewSignedHeaders(method, pathWithQuery string, body []byte, secret string) (SignedHeaders, error) {
	requestId, err := RandomHex(16)
	if err != nil {
		return SignedHeaders{}, err
	}
	nonce, err := RandomHex(16)
	if err != nil {
		return SignedHeaders{}, err
	}
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	bodyHash := Sha256Hex(body)
	canonical := CanonicalString(method, pathWithQuery, timestamp, nonce, bodyHash)
	signature := HmacSha256Base64(secret, canonical)
	return SignedHeaders{
		RequestId: requestId,
		Timestamp: timestamp,
		Nonce:     nonce,
		Signature: signature,
		BodyHash:  bodyHash,
	}, nil
}

func CanonicalString(method, pathWithQuery, timestamp, nonce, bodyHash string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + "\n" +
		pathWithQuery + "\n" +
		timestamp + "\n" +
		nonce + "\n" +
		bodyHash
}

func Sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func HmacSha256Base64(secret, canonical string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func RandomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

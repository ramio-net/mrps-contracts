package protocol

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	HeaderInstallation = "X-MRPS-Installation"
	HeaderTimestamp    = "X-MRPS-Timestamp"
	HeaderNonce        = "X-MRPS-Nonce"
	HeaderSignature    = "X-MRPS-Signature"
)

func SecretKey(edgeSecret string) []byte {
	sum := sha256.Sum256([]byte(edgeSecret))
	return sum[:]
}

func CanonicalRequest(method, path string, timestamp int64, nonce string, body []byte) string {
	sum := sha256.Sum256(body)
	return strings.ToUpper(method) + "\n" + path + "\n" +
		strconv.FormatInt(timestamp, 10) + "\n" + nonce + "\n" + hex.EncodeToString(sum[:])
}

func SignRequest(method, path string, timestamp int64, nonce string, body []byte, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(CanonicalRequest(method, path, timestamp, nonce, body)))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyRequest(method, path string, timestamp int64, nonce string, body []byte, secret []byte, signature string) bool {
	want := SignRequest(method, path, timestamp, nonce, body, secret)
	got, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	wantBytes, _ := base64.StdEncoding.DecodeString(want)
	return hmac.Equal(got, wantBytes)
}

func RandomNonce() (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(rand.Reader, b[:]); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b[:]), nil
}

func SignHTTP(req *http.Request, installationID string, secret []byte, body []byte, now time.Time) error {
	nonce, err := RandomNonce()
	if err != nil {
		return err
	}
	ts := now.UTC().UnixMilli()
	req.Header.Set(HeaderInstallation, installationID)
	req.Header.Set(HeaderTimestamp, strconv.FormatInt(ts, 10))
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, SignRequest(req.Method, req.URL.EscapedPath(), ts, nonce, body, secret))
	return nil
}

func TimestampWithinWindow(tsMillis int64, now time.Time, window time.Duration) error {
	ts := time.UnixMilli(tsMillis)
	if ts.Before(now.Add(-window)) || ts.After(now.Add(window)) {
		return fmt.Errorf("timestamp outside window")
	}
	return nil
}

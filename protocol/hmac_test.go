package protocol

import "testing"

func TestSignVerifyRequest(t *testing.T) {
	secret := []byte("edge-secret")
	body := []byte(`{"hello":"world"}`)
	sig := SignRequest("post", "/edge/v1/sync", 123, "nonce", body, secret)
	if !VerifyRequest("POST", "/edge/v1/sync", 123, "nonce", body, secret, sig) {
		t.Fatal("valid signature rejected")
	}
	if VerifyRequest("POST", "/edge/v1/sync", 123, "nonce", []byte(`{}`), secret, sig) {
		t.Fatal("tampered body accepted")
	}
}

func TestSecretKey(t *testing.T) {
	k1 := SecretKey("edge-secret")
	k2 := SecretKey("edge-secret")
	if string(k1) != string(k2) {
		t.Fatal("derived key is not stable")
	}
	if string(k1) == "edge-secret" {
		t.Fatal("derived key must not equal raw secret")
	}
}

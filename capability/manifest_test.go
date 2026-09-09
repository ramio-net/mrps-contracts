package capability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func roots(t *testing.T, name string) (ed25519.PrivateKey, KeySet) {
	t.Helper()
	seed := sha256.Sum256([]byte(name))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return priv, KeySet{name: priv.Public().(ed25519.PublicKey)}
}

func sampleManifest() *KeyManifest {
	_, signerPub := fixtureKey()
	return &KeyManifest{
		Issuer:      "ramio",
		Environment: "production",
		Revision:    5,
		IssuedAt:    time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC),
		Keys: []ManifestKey{{
			KeyID:     "prod-0",
			PublicKey: base64.StdEncoding.EncodeToString(signerPub),
		}},
	}
}

// The door this design closes: whoever takes the running service must not be able to
// hand venues a key of their choosing. The list is signed by the OFFLINE root, so a
// list signed by the online signer is refused however valid it looks.
func TestManifestSignedByTheOnlineSignerIsRefused(t *testing.T) {
	signerPriv, _ := fixtureKey()
	_, rootSet := roots(t, "root-0")

	raw, err := SignManifest(sampleManifest(), "root-0", signerPriv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyManifest(raw, rootSet, "production", 0); err == nil {
		t.Fatal("a key list signed by the online signer was accepted — a compromised " +
			"service could then install any key and mint any entitlement")
	}
}

func TestManifestRoundTrip(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	raw, err := SignManifest(sampleManifest(), "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	m, err := VerifyManifest(raw, rootSet, "production", 4)
	if err != nil {
		t.Fatal(err)
	}
	set, err := m.VerifyKeySet()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set["prod-0"]; !ok {
		t.Fatalf("signer key missing from the resulting set: %v", set)
	}
}

// Replaying an older list is how a withdrawn key gets reinstated.
func TestOlderRevisionIsRefused(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	raw, err := SignManifest(sampleManifest(), "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyManifest(raw, rootSet, "production", 5); err == nil {
		t.Fatal("a manifest that does not advance the revision must be refused")
	}
}

// A manifest built for a test rig must never apply to a venue in the field.
func TestEnvironmentMustMatch(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	m := sampleManifest()
	m.Environment = "staging"
	raw, err := SignManifest(m, "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyManifest(raw, rootSet, "production", 0); err == nil ||
		!strings.Contains(err.Error(), "environment") {
		t.Fatalf("want an environment refusal, got %v", err)
	}
}

// Retiring a key and revoking it are different acts. A retired key still verifies what
// it signed while current; dropping it early would put a hidden expiry on free sets
// that are supposed to have none.
func TestRetiredKeyStillVerifiesRevokedDoesNot(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	_, signerPub := fixtureKey()
	past := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	m := sampleManifest()
	m.Keys = []ManifestKey{
		{KeyID: "prod-0", PublicKey: base64.StdEncoding.EncodeToString(signerPub), SigningUntil: &past},
		{KeyID: "prod-bad", PublicKey: base64.StdEncoding.EncodeToString(signerPub), Revoked: true},
	}
	raw, err := SignManifest(m, "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyManifest(raw, rootSet, "production", 0)
	if err != nil {
		t.Fatal(err)
	}
	set, err := verified.VerifyKeySet()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set["prod-0"]; !ok {
		t.Fatal("a retired key must still verify documents it signed while current")
	}
	if _, ok := set["prod-bad"]; ok {
		t.Fatal("a revoked key must verify nothing at all")
	}
}

// An empty list is an outage, not a policy.
func TestEmptyManifestIsRefused(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	m := sampleManifest()
	m.Keys = nil
	raw, err := SignManifest(m, "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyManifest(raw, rootSet, "production", 0); err == nil {
		t.Fatal("a manifest with no keys would leave a venue unable to verify anything")
	}
}

package capability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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

func signerB64() string {
	_, pub := fixtureKey()
	return base64.StdEncoding.EncodeToString(pub)
}

func sampleManifest() *KeyManifest {
	return &KeyManifest{
		Issuer:      "ramio",
		Environment: "production",
		Revision:    5,
		IssuedAt:    time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC),
		Keys: []ManifestKey{{
			KeyID:                 "prod-0",
			PublicKey:             signerB64(),
			AllowedSchemaVersions: []int{SchemaVersion3},
		}},
	}
}

// mustSignedManifest signs and verifies in one step, for the cases where neither is
// the thing under test.
func mustTrusted(t *testing.T, m *KeyManifest) TrustedKeys {
	t.Helper()
	rootPriv, rootSet := roots(t, "root-0")
	raw, err := SignManifest(m, "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyManifest(raw, rootSet, "production", 0)
	if err != nil {
		t.Fatal(err)
	}
	set, err := verified.Trusted()
	if err != nil {
		t.Fatal(err)
	}
	return set
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
	set, err := m.Trusted()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := set["prod-0"]; !ok {
		t.Fatalf("signer key missing from the resulting set: %v", set)
	}
}

// Replaying an older list is how a withdrawn key gets reinstated. The refusal is
// named, because repeating the CURRENT manifest is the ordinary case on every sync
// and a consumer that read it as a fault would drop trust for no reason.
func TestNotNewerRevisionIsNamedNotFatal(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	raw, err := SignManifest(sampleManifest(), "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	for _, have := range []int64{5, 6} {
		_, err := VerifyManifest(raw, rootSet, "production", have)
		if !errors.Is(err, ErrManifestNotNewer) {
			t.Fatalf("have=%d: want ErrManifestNotNewer so a consumer can keep its "+
				"current set, got %v", have, err)
		}
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
	past := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	m := sampleManifest()
	m.Keys = []ManifestKey{
		{KeyID: "prod-0", PublicKey: signerB64(),
			AllowedSchemaVersions: []int{SchemaVersion3}, SigningUntil: &past},
		{KeyID: "prod-bad", PublicKey: signerB64(),
			AllowedSchemaVersions: []int{SchemaVersion3}, Revoked: true},
	}
	set := mustTrusted(t, m)
	if _, ok := set["prod-0"]; !ok {
		t.Fatal("a retired key must still verify documents it signed while current")
	}
	if _, ok := set["prod-bad"]; ok {
		t.Fatal("a revoked key must verify nothing at all")
	}
}

// An empty list is an outage, not a policy.
func TestEmptyManifestIsRefused(t *testing.T) {
	rootPriv, _ := roots(t, "root-0")
	m := sampleManifest()
	m.Keys = nil
	if _, err := SignManifest(m, "root-0", rootPriv); err == nil {
		t.Fatal("a manifest with no keys would leave a venue unable to verify anything")
	}
}

// Cloud's probe: the manifest restricts this key to schema 2, and a schema 3 document
// signed by it must be refused. The earlier shape dropped the restriction on the way
// from the manifest to the verifier and accepted the document.
func TestKeyMayNotSignASchemaItWasNotAllowed(t *testing.T) {
	m := sampleManifest()
	m.Keys[0].AllowedSchemaVersions = []int{2}
	set := mustTrusted(t, m)

	priv, _ := fixtureKey()
	doc, err := SignV3(sampleV3(), "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyV3(doc, set); err == nil ||
		!strings.Contains(err.Error(), "not permitted to sign schema 3") {
		t.Fatalf("want a schema-permission refusal, got %v", err)
	}
}

// The signing window bounds NEW documents and is measured against the document's own
// signed issued_at, never the reader's clock. Both halves are asserted here: a
// document from inside the window verifies for good, one dated after it does not.
func TestSigningWindowBoundsIssuanceNotVerification(t *testing.T) {
	closed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	m := sampleManifest()
	m.Keys[0].SigningUntil = &closed
	set := mustTrusted(t, m)
	priv, _ := fixtureKey()

	inside := sampleV3()
	inside.IssuedAt = closed.Add(-24 * time.Hour)
	good, err := SignV3(inside, "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyV3(good, set); err != nil {
		t.Fatalf("a document issued while the key was current must keep verifying: %v", err)
	}

	after := sampleV3()
	after.IssuedAt = closed.Add(24 * time.Hour)
	late, err := SignV3(after, "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyV3(late, set); err == nil ||
		!strings.Contains(err.Error(), "stopped being allowed to sign") {
		t.Fatalf("want a signing-window refusal, got %v", err)
	}
}

// Cloud's probe: one key_id present twice, once current and once revoked. Whether the
// key survives would depend on which entry the reader applied last, so the whole
// manifest is refused. Revocation that can be undone by appending a duplicate is not
// revocation.
func TestDuplicateKeyIDMakesTheWholeManifestInvalid(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	m := sampleManifest()
	m.Keys = append(m.Keys, ManifestKey{KeyID: "prod-0", PublicKey: signerB64(),
		AllowedSchemaVersions: []int{SchemaVersion3}, Revoked: true})

	if _, err := SignManifest(m, "root-0", rootPriv); err == nil ||
		!strings.Contains(err.Error(), "more than once") {
		t.Fatalf("signing a manifest with a duplicate key_id must fail, got %v", err)
	}

	// And a manifest that reached a venue with a duplicate anyway — signed by an
	// older tool, say — is refused on the way in rather than resolved by luck.
	rootPriv2, rootSet2 := rootPriv, rootSet
	forced := *sampleManifest()
	forced.FormatVersion = 1
	forced.RootKeyID = "root-0"
	forced.Keys = m.Keys
	raw := mustSignWithoutValidation(t, &forced, rootPriv2)
	if _, err := VerifyManifest(raw, rootSet2, "production", 0); err == nil ||
		!strings.Contains(err.Error(), "more than once") {
		t.Fatalf("want a duplicate refusal at verification, got %v", err)
	}
}

// A manifest whose keys are all revoked must be distinguishable from one that could
// not be read: the first means stop trusting what you have, the second means keep it.
func TestAllRevokedIsItsOwnAnswer(t *testing.T) {
	rootPriv, rootSet := roots(t, "root-0")
	m := sampleManifest()
	m.Keys = []ManifestKey{{KeyID: "prod-0", PublicKey: signerB64(), Revoked: true}}
	raw, err := SignManifest(m, "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := VerifyManifest(raw, rootSet, "production", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verified.Trusted(); !errors.Is(err, ErrAllKeysRevoked) {
		t.Fatalf("want ErrAllKeysRevoked so a consumer drops trust instead of keeping "+
			"the previous set, got %v", err)
	}
}

// A key of the wrong length must come back as an error. ed25519.Verify panics on one,
// and a configuration fault that crashes the process cannot be reported by the venue
// that hit it.
func TestMalformedPublicKeyIsAnErrorNotAPanic(t *testing.T) {
	priv, _ := fixtureKey()
	doc, err := SignV3(sampleV3(), "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	set := TrustedKeys{"prod-0": {
		PublicKey:             ed25519.PublicKey{1},
		AllowedSchemaVersions: []int{SchemaVersion3},
	}}
	if _, err := VerifyV3(doc, set); err == nil ||
		!strings.Contains(err.Error(), "want 32") {
		t.Fatalf("want a key-length error, got %v", err)
	}
}

// A root of the wrong length must not panic either.
func TestMalformedRootKeyIsAnErrorNotAPanic(t *testing.T) {
	rootPriv, _ := roots(t, "root-0")
	raw, err := SignManifest(sampleManifest(), "root-0", rootPriv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyManifest(raw, KeySet{"root-0": {1}}, "production", 0); err == nil ||
		!strings.Contains(err.Error(), "want 32") {
		t.Fatalf("want a root-length error, got %v", err)
	}
}

// A permission that defaults to open is a permission that leaks.
func TestKeyWithNoAllowedSchemaVersionsIsRefusedAtAuthoring(t *testing.T) {
	rootPriv, _ := roots(t, "root-0")
	m := sampleManifest()
	m.Keys[0].AllowedSchemaVersions = nil
	if _, err := SignManifest(m, "root-0", rootPriv); err == nil ||
		!strings.Contains(err.Error(), "no allowed schema versions") {
		t.Fatalf("want an authoring refusal, got %v", err)
	}
}

// mustSignWithoutValidation produces bytes the authoring checks would have refused, to
// prove the verifying side does not rely on them.
func mustSignWithoutValidation(t *testing.T, m *KeyManifest, root ed25519.PrivateKey) []byte {
	t.Helper()
	m.Signature = ""
	unsigned, err := jsonMarshal(m)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalManifestBytes(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(root, signingInput(signingDomainManifest, canonical)))
	raw, err := jsonMarshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

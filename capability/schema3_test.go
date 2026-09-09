package capability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func testKey(t *testing.T, name string) (ed25519.PrivateKey, KeySet) {
	t.Helper()
	seed := sha256.Sum256([]byte(name))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return priv, KeySet{name: priv.Public().(ed25519.PublicKey)}
}

func sampleV3() *ProfileV3 {
	issued := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	return &ProfileV3{
		ProfileID: "9f1e2d3c-0000-4000-8000-000000000001",
		Revision:  7,
		Subject: SubjectV3{
			InstallationID: "6d79cb18-0da4-4f11-9e6b-f6defe8e211e",
			EdgeID:         "152b9f17-5410-4178-a4d3-d00a62c39559",
			OrgID:          "10000000-0000-4000-8000-000000000001",
		},
		IssuedAt:         issued,
		OfflineWindowSec: 7 * 24 * 3600,
		Registered: CapabilitySetV3{
			Limits:             Limits{MaxCameras: 4, ConfigEditable: false},
			Features:           Features{TelemetryUpload: true, MRPSConsoleSync: true},
			ConditionsRevision: 3,
		},
		Full: &PaidCapabilitySetV3{
			Limits:             Limits{MaxCameras: 8, ConfigEditable: true},
			Features:           Features{TelemetryUpload: true, MRPSConsoleSync: true, MultiStream: true},
			ConditionsRevision: 3,
			ValidFrom:          issued,
			ValidUntil:         issued.Add(30 * 24 * time.Hour),
		},
	}
}

func TestSignAndVerifyV3(t *testing.T) {
	priv, keys := testKey(t, "prod-0")
	raw, err := SignV3(sampleV3(), "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyV3(raw, keys)
	if err != nil {
		t.Fatalf("a document we just signed must verify: %v", err)
	}
	if got.Registered.Limits.MaxCameras != 4 || got.Full.Limits.MaxCameras != 8 {
		t.Fatalf("sets came through wrong: %+v", got)
	}
}

// The reason canonicalisation reads the received bytes instead of rebuilding the
// document from a Go type. A field added to a later build must stay inside the
// signature, or every venue that has not been updated starts rejecting valid
// documents — the worst possible failure, because it hits the venues that are
// furthest behind and least able to be fixed remotely.
func TestUnknownFieldStaysInsideTheSignature(t *testing.T) {
	priv, keys := testKey(t, "prod-0")
	raw, err := SignV3(sampleV3(), "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}

	// Cloud of a later version signs a document carrying a field this build has
	// never heard of. Re-sign with the extension present, exactly as a newer issuer
	// would produce it.
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	obj["talkback_policy"] = map[string]any{"enabled": true, "channels": 2}
	delete(obj, "signature")
	unsigned, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalBytesV3(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	obj["signature"] = signB64(priv, SigningInputV3(canonical))
	extended, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := VerifyV3(extended, keys); err != nil {
		t.Fatalf("an older build must still verify a document carrying a field it "+
			"does not know: %v", err)
	}

	// And the extension must be covered: changing it has to break the signature.
	obj["talkback_policy"] = map[string]any{"enabled": true, "channels": 99}
	tampered, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyV3(tampered, keys); err == nil {
		t.Fatal("an unknown field was not covered by the signature — anything a build " +
			"does not understand could then be edited in flight")
	}
}

// {"a":1,"a":2} means different things to different decoders. A document the signer
// and the verifier read differently is not a document.
func TestDuplicateKeysAreRejected(t *testing.T) {
	priv, keys := testKey(t, "prod-0")
	raw, err := SignV3(sampleV3(), "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	doubled := strings.Replace(string(raw), `"revision":7`, `"revision":7,"revision":9`, 1)
	if doubled == string(raw) {
		t.Fatal("test did not inject a duplicate")
	}
	if _, err := VerifyV3([]byte(doubled), keys); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate keys must be refused by name, got %v", err)
	}
}

// Every field except the signature is covered, including the ones that say what the
// document IS. A document must not be re-readable as another schema or attributable
// to another key without breaking.
func TestSchemaAndKeyAreSigned(t *testing.T) {
	priv, keys := testKey(t, "prod-0")
	raw, err := SignV3(sampleV3(), "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	for _, edit := range []struct{ name, from, to string }{
		{"schema", `"schema_version":3`, `"schema_version":4`},
		{"key", `"key_id":"prod-0"`, `"key_id":"prod-1"`},
		{"free cameras", `"max_cameras":4`, `"max_cameras":40`},
		{"revision", `"revision":7`, `"revision":8`},
	} {
		t.Run(edit.name, func(t *testing.T) {
			tampered := strings.Replace(string(raw), edit.from, edit.to, 1)
			if tampered == string(raw) {
				t.Fatalf("edit %q did not apply", edit.name)
			}
			if _, err := VerifyV3([]byte(tampered), keys); err == nil {
				t.Fatalf("editing %s left the document acceptable", edit.name)
			}
		})
	}
}

// Rotation without overlap is an outage. Both keys are trusted while the fleet moves.
func TestBothKeysVerifyDuringRotation(t *testing.T) {
	priv0, set0 := testKey(t, "prod-0")
	priv1, set1 := testKey(t, "prod-1")
	during := KeySet{}
	for k, v := range set0 {
		during[k] = v
	}
	for k, v := range set1 {
		during[k] = v
	}

	oldDoc, err := SignV3(sampleV3(), "prod-0", priv0)
	if err != nil {
		t.Fatal(err)
	}
	newDoc, err := SignV3(sampleV3(), "prod-1", priv1)
	if err != nil {
		t.Fatal(err)
	}
	for name, doc := range map[string][]byte{"old key": oldDoc, "new key": newDoc} {
		if _, err := VerifyV3(doc, during); err != nil {
			t.Fatalf("%s must verify during rotation: %v", name, err)
		}
	}
	// And a node that only knows the new key must refuse the old one by name rather
	// than call it forged.
	if _, err := VerifyV3(oldDoc, set1); err == nil || !strings.Contains(err.Error(), "unknown key_id") {
		t.Fatalf("want an unknown-key diagnosis, got %v", err)
	}
}

// The document exists so that the free set survives the paid one. One without a
// usable free block would leave the step down nowhere to land.
func TestDocumentWithoutAFreeSetIsRefused(t *testing.T) {
	priv, keys := testKey(t, "prod-0")
	p := sampleV3()
	p.Registered.Limits.MaxCameras = 0
	raw, err := SignV3(p, "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyV3(raw, keys); err == nil {
		t.Fatal("a document with no usable registered set must not be accepted")
	}
}

// The deadline is the earlier of what was bought and how stale the assertion may get.
func TestPaidDeadlineTakesTheEarlier(t *testing.T) {
	p := *sampleV3()
	// Thirty days bought, seven days of autonomy: the window wins.
	deadline, ok := p.PaidUntil()
	if !ok {
		t.Fatal("a paid document must report a deadline")
	}
	if want := p.IssuedAt.Add(7 * 24 * time.Hour); !deadline.Equal(want) {
		t.Fatalf("deadline = %v, want the offline window at %v", deadline, want)
	}

	// Two days bought, seven days of autonomy: what was bought wins.
	p.Full.ValidUntil = p.IssuedAt.Add(48 * time.Hour)
	deadline, _ = p.PaidUntil()
	if !deadline.Equal(p.Full.ValidUntil) {
		t.Fatalf("deadline = %v, want the purchase at %v", deadline, p.Full.ValidUntil)
	}

	// Equality with the deadline counts as expired, not as the last good moment.
	if p.PaidActiveAt(deadline) {
		t.Fatal("standing exactly on the deadline must read as expired")
	}
	if !p.PaidActiveAt(deadline.Add(-time.Second)) {
		t.Fatal("a second before the deadline must still be paid")
	}
}

// A free document is not a broken one. No paid set is the normal state of the tier
// everybody lands on, and it must not read as an error anywhere.
func TestFreeOnlyDocumentIsValid(t *testing.T) {
	priv, keys := testKey(t, "prod-0")
	p := sampleV3()
	p.Full = nil
	raw, err := SignV3(p, "prod-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyV3(raw, keys)
	if err != nil {
		t.Fatalf("a free-only document must verify: %v", err)
	}
	if _, ok := got.PaidUntil(); ok {
		t.Fatal("a free-only document must not claim a paid deadline")
	}
	if got.PaidActiveAt(got.IssuedAt) {
		t.Fatal("and must never report the paid set as active")
	}
}

// Canonical form must not depend on how the sender happened to order its keys.
func TestCanonicalFormIgnoresKeyOrder(t *testing.T) {
	a := []byte(`{"b":2,"a":1,"c":{"y":2,"x":1}}`)
	b := []byte(`{"c":{"x":1,"y":2},"a":1,"b":2}`)
	ca, err := CanonicalBytesV3(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := CanonicalBytesV3(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(ca) != string(cb) {
		t.Fatalf("canonical forms differ:\n%s\n%s", ca, cb)
	}
}

func signB64(priv ed25519.PrivateKey, payload []byte) string {
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
}

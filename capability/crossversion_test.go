package capability

import (
	"crypto/ed25519"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A schema 2 profile signed by v0.6.0 must still verify. Forever.
//
// This file exists because it did not, for one day. Introducing schema 3 brought a
// stricter canonical form, and the same helper was reused for schema 2 — so a profile
// with an ampersand in its key_id verified under v0.6.0 and was refused afterwards.
// Cloud caught it with a probe running one program against both versions.
//
// The reasoning that let it through is worth keeping too: schema 2 has no free-text
// field, so no string in it could contain an ampersand — which was true of the
// documents we happened to have, and never true of the contract. The golden vector
// passed for exactly that reason. A test that agrees by accident is the thing these
// vectors exist to prevent, and it caught nothing here because it did not contain the
// character in question.
//
// The document below was produced by Cloud on v0.6.0 with a throwaway key (a
// zero-byte seed, trusted by nothing) and is used unchanged. It is evidence from
// another build, which is what makes it worth more than anything this build can
// generate about its own past.
func TestSchema2ProfileFromV060StillVerifies(t *testing.T) {
	raw, err := os.ReadFile("testdata/schema2_v060_profile.json")
	if err != nil {
		t.Fatal(err)
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.KeyID, "&") {
		t.Fatal("this vector is only worth having because its key_id contains an " +
			"ampersand; without it the test passes while proving nothing")
	}
	throwaway := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	if err := Verify(p, KeySet{p.KeyID: throwaway.Public().(ed25519.PublicKey)}); err != nil {
		t.Fatalf("a profile signed by v0.6.0 no longer verifies: %v — the schema 2 "+
			"canonical form has moved, and every signature already in the field with it", err)
	}
}

// The two codecs differ on purpose, and the difference is asserted rather than left
// to be rediscovered. If these ever agree, one of them has drifted.
func TestTheTwoCodecsAreDeliberatelyDifferent(t *testing.T) {
	const withAmpersand = `{"a":"x&y"}`

	strict, err := canonicalWithout([]byte(withAmpersand), "signature")
	if err != nil {
		t.Fatal(err)
	}
	if got := string(strict); got != `{"a":"x&y"}` {
		t.Fatalf("schema 3 must emit the ampersand literally, got %s", got)
	}

	legacy, err := legacyCanonicalDoc(withAmpersand)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(legacy); got != `{"a":"x\u0026y"}` {
		t.Fatalf("schema 2 must keep Go's HTML escaping of the ampersand, got %s", got)
	}
}

// Numbers too: schema 2 tolerated what schema 3 refuses, and must go on tolerating it.
func TestSchema2StillAcceptsWhatSchema3Refuses(t *testing.T) {
	for _, doc := range []string{`{"a":1.5}`, `{"a":18446744073709551615}`, `{"ключ":1}`} {
		if _, err := legacyCanonicalDoc(doc); err != nil {
			t.Fatalf("schema 2 must still canonicalise %s: %v", doc, err)
		}
		if _, err := canonicalWithout([]byte(doc), "signature"); err == nil {
			t.Fatalf("schema 3 must refuse %s", doc)
		}
	}
}

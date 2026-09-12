package capability

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
)

// leniently canonicalises the way an implementation built on an ordinary JSON parser
// would: whatever the parser returns, formatted by the published rules. It performs
// none of the raw-text checks, because a parser cannot — by the time it returns, the
// duplicate is gone, 7e0 has become 7, and the lone surrogate has become U+FFFD.
func leniently(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	// No UseNumber: this is what JSON.parse gives you, a float64 with the original
	// spelling discarded.
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("lenient parse: %v", err)
	}
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatal("not an object")
	}
	delete(obj, "signature")
	var out bytes.Buffer
	if err := writeLenient(&out, obj); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func writeLenient(out *bytes.Buffer, v any) error {
	if f, ok := v.(float64); ok {
		// What String(n) does in JavaScript for an integral value.
		out.WriteString(strconv.FormatFloat(f, 'f', -1, 64))
		return nil
	}
	return writeCanonical(out, normaliseNumbers(v), codecSigned)
}

func normaliseNumbers(v any) any {
	switch x := v.(type) {
	case float64:
		return json.Number(strconv.FormatFloat(x, 'f', -1, 64))
	case []any:
		for i := range x {
			x[i] = normaliseNumbers(x[i])
		}
		return x
	case map[string]any:
		for k := range x {
			x[k] = normaliseNumbers(x[k])
		}
		return x
	}
	return v
}

// TestTheRawChecksAreLoadBearing is the evidence behind the mandatory wording in
// _implementers_note.
//
// Cloud raised this before the tag: the note used to say these gaps could not cause a
// wrong ACCEPT, only a differently-worded refusal. That was false, and false in the
// direction that matters — it read as permission to skip the checks. Each vector below
// carries a GENUINE signature over the canonical form the issuer signed, while the
// bytes on the wire say something else. A verifier that trusts its parser accepts them.
//
// The test asserts both halves: the lenient path verifies, and this implementation
// refuses. If the first half ever stops holding, the vector has lost its point and
// should be rebuilt rather than deleted.
func TestTheRawChecksAreLoadBearing(t *testing.T) {
	priv, pub := fixtureKey()
	keys := DevelopmentTrust("fixture-0", pub, SchemaVersion3)
	signed, err := SignV3(sampleV3(), "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"duplicate_key_shadowing":   true,
		"exponent_preserving_value": true,
		"surrogate_rewrite":         true,
	}
	seen := map[string]bool{}

	for _, v := range negativeVectors(t, signed) {
		if !want[v.Name] {
			continue
		}
		seen[v.Name] = true
		t.Run(v.Name, func(t *testing.T) {
			var probe struct {
				Signature string `json:"signature"`
			}
			if err := json.Unmarshal([]byte(v.Document), &probe); err != nil {
				t.Fatal(err)
			}
			sig, err := base64.StdEncoding.DecodeString(probe.Signature)
			if err != nil {
				t.Fatal(err)
			}
			canonical := leniently(t, []byte(v.Document))
			if !ed25519.Verify(pub, SigningInputV3(canonical), sig) {
				t.Fatalf("vector %q no longer demonstrates anything: a lenient verifier "+
					"already refuses it, so it proves nothing about why the raw checks "+
					"are required", v.Name)
			}
			if _, err := VerifyV3([]byte(v.Document), keys); err == nil {
				t.Fatalf("vector %q was ACCEPTED by this implementation — the raw check "+
					"it exists to exercise is not running", v.Name)
			}
		})
	}

	for name := range want {
		if !seen[name] {
			t.Fatalf("vector %q is missing from the published set", name)
		}
	}
}

package capability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// updateFixtures regenerates testdata/schema3_fixtures.json. Run with:
//
//	go test ./capability/ -run Fixtures -update
var updateFixtures = flag.Bool("update", false, "rewrite the schema 3 fixture file")

const fixturePath = "testdata/schema3_fixtures.json"

// fixtureSeed makes the TEST key reproducible from nothing but this repository.
//
// It is a test key and only a test key. Nothing signed with it may ever be accepted
// by a production verifier — that is what the environment check on the Cloud signer
// is for, and what the removal of the development key from issuance enforces.
const fixtureSeed = "mrps-capability-schema-3-fixture-key-v1"

type schema3Fixtures struct {
	Note string `json:"_note"`
	Key  struct {
		KeyID        string `json:"key_id"`
		Seed         string `json:"private_seed_utf8"`
		PublicKeyB64 string `json:"public_key_base64"`
	} `json:"key"`
	Valid struct {
		Document      json.RawMessage `json:"document"`
		CanonicalJSON string          `json:"canonical_json"`
		SigningInput  string          `json:"signing_input_hex"`
		Signature     string          `json:"signature_base64"`
	} `json:"valid"`
	Negative []negativeVector `json:"negative"`
}

type negativeVector struct {
	Name string `json:"name"`
	Why  string `json:"why"`
	// Document is the LITERAL bytes to feed a verifier, carried as a string because
	// some vectors are deliberately malformed JSON and could not survive being stored
	// as a JSON value.
	Document string `json:"document"`
}

func fixtureKey() (ed25519.PrivateKey, ed25519.PublicKey) {
	seed := sha256.Sum256([]byte(fixtureSeed))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return priv, priv.Public().(ed25519.PublicKey)
}

// TestSchema3Fixtures is the cross-implementation contract for schema 3.
//
// Cloud signs, Edge verifies, and the two agree only if they canonicalise the same
// bytes the same way. Prose cannot settle that — field order, number formatting, how
// an absent value differs from a false one. These vectors can, and they are checked
// on every run so a change to the encoder cannot pass unnoticed.
func TestSchema3Fixtures(t *testing.T) {
	priv, pub := fixtureKey()

	doc := sampleV3()
	doc.SchemaVersion = SchemaVersion3
	doc.KeyID = "fixture-0"
	signed, err := SignV3(doc, "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalBytesV3(signed)
	if err != nil {
		t.Fatal(err)
	}

	var want schema3Fixtures
	want.Note = "TEST VECTORS. The key below is derived from a constant in this " +
		"repository and must never be trusted by a production verifier."
	want.Key.KeyID = "fixture-0"
	want.Key.Seed = fixtureSeed
	want.Key.PublicKeyB64 = base64.StdEncoding.EncodeToString(pub)
	want.Valid.Document = json.RawMessage(signed)
	want.Valid.CanonicalJSON = string(canonical)
	want.Valid.SigningInput = hex.EncodeToString(SigningInputV3(canonical))
	want.Valid.Signature = doc.Signature
	want.Negative = negativeVectors(t, signed)

	encoded, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')

	if *updateFixtures {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixturePath, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", fixturePath)
		return
	}

	onDisk, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("%v — run: go test ./capability/ -run Fixtures -update", err)
	}
	if strings.ReplaceAll(string(onDisk), "\r\n", "\n") != string(encoded) {
		t.Fatal("the published vectors no longer match what this build produces. " +
			"If the encoder changed deliberately this is a WIRE BREAK: every venue " +
			"holding a document signed under the old form stops verifying. Regenerate " +
			"only together with a version bump and a deployment order.")
	}
}

// negativeVectors are the refusals both sides must agree on. A verifier that accepts
// any of these is not interoperable, however well it handles the valid case.
func negativeVectors(t *testing.T, signed []byte) []negativeVector {
	t.Helper()
	out := []negativeVector{}
	add := func(name, why string, doc []byte) {
		out = append(out, negativeVector{Name: name, Why: why, Document: string(doc)})
	}

	add("tampered_free_cameras",
		"the free set is inside the signature; raising it must break verification",
		[]byte(strings.Replace(string(signed), `"max_cameras":4`, `"max_cameras":40`, 1)))
	add("tampered_key_id",
		"key_id is signed, so a document cannot be re-attributed to another key",
		[]byte(strings.Replace(string(signed), `"key_id":"fixture-0"`, `"key_id":"other"`, 1)))
	add("tampered_schema_version",
		"schema_version is signed, so a document cannot be re-read as another schema",
		[]byte(strings.Replace(string(signed), `"schema_version":3`, `"schema_version":4`, 1)))
	add("duplicate_key",
		"decoders disagree about which value wins, so the document has no single meaning",
		[]byte(strings.Replace(string(signed), `"revision":7`, `"revision":7,"revision":9`, 1)))
	add("missing_signature",
		"an unsigned document grants nothing, including the free set it carries",
		[]byte(strings.Replace(string(signed), `"signature":"`+extractSignature(signed)+`"`, `"signature":""`, 1)))

	// Signed PROPERLY with an unusable free set, so the refusal comes from the missing
	// set and not from a broken signature. Cloud's independent verifier caught the
	// first version of this vector: it was built by deleting the block from an already
	// signed document, so it failed on the signature and proved nothing about the rule
	// it was named for. A vector that passes for the wrong reason is worse than none —
	// it reports agreement that was never tested.
	priv, _ := fixtureKey()
	empty := sampleV3()
	empty.Registered.Limits.MaxCameras = 0
	unusable, err := SignV3(empty, "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	add("no_usable_registered_set",
		"signature is VALID; the refusal must come from the free set being unusable, "+
			"because without it the step down after expiry has nowhere to land",
		unusable)

	// And separately: content removed after signing. This one must fail on the
	// signature, and says so.
	var stripped map[string]any
	if err := json.Unmarshal(signed, &stripped); err != nil {
		t.Fatal(err)
	}
	delete(stripped, "registered")
	raw, err := json.Marshal(stripped)
	if err != nil {
		t.Fatal(err)
	}
	add("registered_removed_after_signing",
		"the free set is inside the signature, so removing it must break verification",
		raw)

	add("trailing_content",
		"anything after the document is refused; a stray closing brace is not "+
			"another value and must not be read as one",
		[]byte(string(signed)+"}"))

	return out
}

func extractSignature(signed []byte) string {
	var p ProfileV3
	_ = json.Unmarshal(signed, &p)
	return p.Signature
}

// Every negative vector must actually be refused. Publishing a vector that the
// reference implementation accepts would be worse than publishing none.
func TestNegativeVectorsAreAllRefused(t *testing.T) {
	priv, pub := fixtureKey()
	keys := KeySet{"fixture-0": pub}

	doc := sampleV3()
	signed, err := SignV3(doc, "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range negativeVectors(t, signed) {
		t.Run(v.Name, func(t *testing.T) {
			if _, err := VerifyV3([]byte(v.Document), keys); err == nil {
				t.Fatalf("vector %q was accepted; %s", v.Name, v.Why)
			}
		})
	}
}

// The valid vector must verify, and its stored deadline must be what the two sides
// will compute independently.
func TestValidVectorVerifies(t *testing.T) {
	priv, pub := fixtureKey()
	doc := sampleV3()
	signed, err := SignV3(doc, "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	got, err := VerifyV3(signed, KeySet{"fixture-0": pub})
	if err != nil {
		t.Fatal(err)
	}
	deadline, ok := got.PaidUntil()
	if !ok {
		t.Fatal("the paid vector must report a deadline")
	}
	if want := got.IssuedAt.Add(7 * 24 * time.Hour); !deadline.Equal(want) {
		t.Fatalf("deadline = %v, want %v", deadline, want)
	}
}

package capability

import (
	"bytes"
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

// fixtureRootSeed is the OFFLINE ROOT of the test vectors: it signs key manifests and
// nothing else. Separate from the signer above, because the whole point of the
// manifest is that the two keys are not the same key.
const fixtureRootSeed = "mrps-signing-key-manifest-fixture-root-v1"

// Stages a vector is refused at. A verifier that refuses the right document at the
// wrong stage has agreed by accident; naming the stage is what turns "both sides said
// no" into "both sides said no for the same reason".
const (
	stageVerifyDocument = "verify_document"
	stageVerifyManifest = "verify_manifest"
	stageTrusted        = "trusted_key_set"
)

type schema3Fixtures struct {
	Note string `json:"_note"`
	// Implementers records what a verifier built on an ordinary JSON parser must add
	// for itself. Measured against Node: every POSITIVE vector here canonicalises to
	// identical bytes in JavaScript, but several refusals are invisible to JSON.parse,
	// and a reader who does not know that will believe they have an agreement they
	// have not got.
	Implementers []string `json:"_implementers_note"`
	Key          struct {
		KeyID        string `json:"key_id"`
		Seed         string `json:"private_seed_utf8"`
		PublicKeyB64 string `json:"public_key_base64"`
	} `json:"key"`
	Valid signedVector `json:"valid"`
	// Codec are documents that must canonicalise to exactly these bytes. They exist
	// because the valid vector above is all ASCII integers and would not notice the
	// places two implementations actually diverge.
	Codec    []signedVector   `json:"codec"`
	Negative []negativeVector `json:"negative"`
	Manifest manifestFixtures `json:"manifest"`
}

type signedVector struct {
	Name string `json:"name,omitempty"`
	Why  string `json:"why,omitempty"`
	// Document is the LITERAL bytes to feed a verifier, carried as a string because
	// some vectors are deliberately malformed JSON and could not survive being stored
	// as a JSON value.
	Document      string `json:"document"`
	CanonicalJSON string `json:"canonical_json"`
	SigningInput  string `json:"signing_input_hex"`
	Signature     string `json:"signature_base64"`
}

type negativeVector struct {
	Name string `json:"name"`
	Why  string `json:"why"`
	// Stage says where the refusal must happen, and ExpectedError what it must say.
	// Cloud asked for both: their first review found a vector that was refused for a
	// reason unrelated to the rule it was named for, and neither side noticed because
	// the only assertion was that something failed.
	Stage         string `json:"stage"`
	ExpectedError string `json:"expected_error"`
	Document      string `json:"document"`
}

type manifestFixtures struct {
	RootKey struct {
		KeyID        string `json:"key_id"`
		Seed         string `json:"private_seed_utf8"`
		PublicKeyB64 string `json:"public_key_base64"`
	} `json:"root_key"`
	// Environment is what a verifier must claim to be when checking these vectors.
	Environment string           `json:"environment"`
	Valid       signedVector     `json:"valid"`
	Negative    []manifestReject `json:"negative"`
}

type manifestReject struct {
	Name          string `json:"name"`
	Why           string `json:"why"`
	Stage         string `json:"stage"`
	ExpectedError string `json:"expected_error"`
	Manifest      string `json:"manifest"`
	// HaveRevision is the revision the venue already holds.
	HaveRevision int64 `json:"have_revision"`
	// Document is present only for vectors refused at stageVerifyDocument.
	Document string `json:"document,omitempty"`
}

func fixtureKey() (ed25519.PrivateKey, ed25519.PublicKey) {
	return keyFromSeed(fixtureSeed)
}

func fixtureRoot() (ed25519.PrivateKey, ed25519.PublicKey) {
	return keyFromSeed(fixtureRootSeed)
}

func keyFromSeed(s string) (ed25519.PrivateKey, ed25519.PublicKey) {
	seed := sha256.Sum256([]byte(s))
	priv := ed25519.NewKeyFromSeed(seed[:])
	return priv, priv.Public().(ed25519.PublicKey)
}

// TestSchema3Fixtures is the cross-implementation contract for schema 3.
//
// Cloud signs, Edge verifies, and the two agree only if they canonicalise the same
// bytes the same way. Prose cannot settle that — field order, number formatting, how
// an absent value differs from a false one, whether an ampersand is escaped. These
// vectors can, and they are checked on every run so a change to the encoder cannot
// pass unnoticed.
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
	want.Note = "TEST VECTORS. The keys below are derived from constants in this " +
		"repository and must never be trusted by a production verifier."
	want.Implementers = []string{
		"MANDATORY. The three checks below are not diagnostics and not politeness: a " +
			"verifier that skips them will ACCEPT documents that were never signed. " +
			"They are listed separately only because an ordinary JSON parser cannot " +
			"perform them — by the time it returns, the evidence is gone.",
		"Duplicate object keys: JSON.parse and Go both keep the LAST value, so a forged " +
			"pair placed BEFORE the signed one leaves the parsed value untouched — the " +
			"signature verifies, and any consumer whose parser takes the first value " +
			"instead reads the forged one. Vector duplicate_key_shadowing is a document " +
			"a verifier without a raw duplicate scan accepts. Scan the raw bytes.",
		"Fractions and exponents: after parsing, 7e0 and 7 are the same value, so " +
			"rewriting a signed 7 as 7e0 changes the bytes without changing what the " +
			"parser sees. The signature then verifies over content the issuer never " +
			"produced. Vector exponent_preserving_value shows it. Reject them from the " +
			"RAW text, before parsing.",
		"Unpaired surrogate escapes: Go silently replaces them with U+FFFD, so a signed " +
			"U+FFFD can be rewritten as \\ud800 and still verify. Vector " +
			"surrogate_rewrite shows it. Reject them from the raw text.",
		"Whole numbers stay within ±(2^53−1) so JSON.parse remains exact. This one IS " +
			"sufficient on its own: inside that range no two distinct integers round " +
			"together, which is what removes the same attack for numbers.",
		"The vector NAME is the portable reason code; expected_error is what the Go " +
			"reference prints and will read differently in another language.",
		"Put another way: verifying a signature proves the CANONICAL FORM was signed. " +
			"It says nothing about the bytes that arrived unless the canonical form can " +
			"be reached from those bytes one way only. These checks are what makes that " +
			"true.",
	}
	want.Key.KeyID = "fixture-0"
	want.Key.Seed = fixtureSeed
	want.Key.PublicKeyB64 = base64.StdEncoding.EncodeToString(pub)
	want.Valid = signedVector{
		Document:      string(signed),
		CanonicalJSON: string(canonical),
		SigningInput:  hex.EncodeToString(SigningInputV3(canonical)),
		Signature:     doc.Signature,
	}
	want.Codec = codecVectors(t)
	want.Negative = negativeVectors(t, signed)
	want.Manifest = manifestVectors(t)

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

// fixtureBody is a minimal but ACCEPTABLE document, with room for one extra clause.
// It has to survive VerifyV3 and not only the canonicaliser, or a vector built on it
// proves nothing about the wire.
func fixtureBody(extra string) string {
	return `{"schema_version":3,"profile_id":"p-1","revision":1,` +
		`"subject":{"installation_id":"i-1","edge_id":"e-1","org_id":"o-1"},` +
		`"issued_at":"2026-09-09T12:00:00Z","offline_window_sec":259200,` +
		`"registered":{"limits":{"max_cameras":4,"config_editable":false},` +
		`"features":{"multi_stream":false,"studio_node_transport":false,` +
		`"edge_local_config_ui":true,"mrps_console_sync":true,"remote_control":false,` +
		`"telemetry_upload":true,"operational_profile":false},"conditions_revision":1}` +
		extra + `,"key_id":"fixture-0"}`
}

// codecVectors exercise the parts of the encoder the realistic document never
// reaches. Every one of them is a place where the obvious implementation in another
// language disagrees with the obvious implementation here, measured rather than
// guessed — see the rules at the top of canonical.go.
func codecVectors(t *testing.T) []signedVector {
	t.Helper()
	cases := []struct{ name, why, doc string }{
		{"registered_only",
			"no paid set at all: the free tier is the ordinary case, not a degraded one",
			fixtureBody("")},
		{"unknown_signed_field",
			"a field this build does not know stays inside the signature; rebuilding the " +
				"document from a struct would drop it and break verification exactly at " +
				"the venues furthest behind",
			fixtureBody(`,"future_field":{"nested":[1,2,3]}`)},
		{"explicit_null_and_false",
			"an absent value, an explicit null and a false are three different things " +
				"and must not collapse into one another",
			fixtureBody(`,"optional_thing":null,"another_thing":false`)},
		{"html_sensitive_characters",
			"Go's encoding/json escapes < > and & while JavaScript does not; a venue " +
				"whose name contains an ampersand must not fail verification",
			fixtureBody(`,"note":"Ramio <Sport> & Co"`)},
		{"unicode_text",
			"Cyrillic, an emoji, U+2028 and a non-BMP character, all emitted literally",
			fixtureBody(`,"note":"Кириллица ✂   𝄞"`)},
		{"control_characters",
			"below 0x20 the short escapes are used where JSON defines them and a " +
				"lowercase \\u00xx otherwise",
			fixtureBody(`,"note":"a\tb\nc\u0001d"`)},
		{"safe_integer_bounds",
			"the largest and smallest whole numbers a signed document may carry: JavaScript's exact-integer range, beyond which JSON.parse rounds in silence",
			fixtureBody(`,"big":9007199254740991,"small":-9007199254740991`)},
	}

	priv, pub := fixtureKey()
	keys := DevelopmentTrust("fixture-0", pub, SchemaVersion3)
	out := []signedVector{}
	for _, c := range cases {
		signed := signRawFixture(t, c.doc, priv)
		canonical, err := CanonicalBytesV3(signed)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		// A codec vector that does not verify is not a vector, it is a typo.
		if _, err := VerifyV3(signed, keys); err != nil {
			t.Fatalf("%s must verify: %v", c.name, err)
		}
		var p ProfileV3
		if err := json.Unmarshal(signed, &p); err != nil {
			t.Fatal(err)
		}
		out = append(out, signedVector{
			Name: c.name, Why: c.why,
			Document:      string(signed),
			CanonicalJSON: string(canonical),
			SigningInput:  hex.EncodeToString(SigningInputV3(canonical)),
			Signature:     p.Signature,
		})
	}
	return out
}

// signRawFixture signs bytes as authored, so a vector can carry fields no Go struct
// has. Going through a struct would silently drop exactly what these vectors test.
func signRawFixture(t *testing.T, doc string, priv ed25519.PrivateKey) []byte {
	t.Helper()
	canonical, err := CanonicalBytesV3([]byte(doc))
	if err != nil {
		t.Fatalf("canonicalise fixture: %v", err)
	}
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, SigningInputV3(canonical)))

	dec := json.NewDecoder(bytes.NewReader([]byte(doc)))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		t.Fatal(err)
	}
	obj["signature"] = sig
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// negativeVectors are the refusals both sides must agree on, each with the stage and
// the reason it must be refused for. A verifier that accepts any of these is not
// interoperable, however well it handles the valid case.
func negativeVectors(t *testing.T, signed []byte) []negativeVector {
	t.Helper()
	out := []negativeVector{}
	add := func(name, why, expected string, doc []byte) {
		out = append(out, negativeVector{Name: name, Why: why, Stage: stageVerifyDocument,
			ExpectedError: expected, Document: string(doc)})
	}

	add("tampered_free_cameras",
		"the free set is inside the signature; raising it must break verification",
		"invalid signature",
		[]byte(strings.Replace(string(signed), `"max_cameras":4`, `"max_cameras":40`, 1)))
	add("tampered_key_id",
		"key_id is signed, so a document cannot be re-attributed to another key",
		"unknown key_id",
		[]byte(strings.Replace(string(signed), `"key_id":"fixture-0"`, `"key_id":"other"`, 1)))
	add("tampered_schema_version",
		"schema_version is signed, so a document cannot be re-read as another schema",
		"unsupported schema_version",
		[]byte(strings.Replace(string(signed), `"schema_version":3`, `"schema_version":4`, 1)))
	add("duplicate_key",
		"decoders disagree about which value wins, so the document has no single meaning",
		"duplicate key",
		[]byte(strings.Replace(string(signed), `"revision":7`, `"revision":7,"revision":9`, 1)))
	add("missing_signature",
		"an unsigned document grants nothing, including the free set it carries",
		"missing signature",
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
		"registered set is missing or unusable",
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
		"invalid signature",
		raw)

	add("trailing_content",
		"anything after the document is refused; a stray closing brace is not "+
			"another value and must not be read as one",
		"trailing content",
		[]byte(string(signed)+"}"))

	// The three below are the ones that matter most, and the reason the raw-text
	// checks are mandatory rather than advisory. Cloud raised it before the tag and
	// they were right: each of these carries a GENUINE signature over the signed
	// canonical form, while the bytes on the wire say something the issuer never
	// produced. A verifier that trusts its JSON parser accepts all three.
	//
	// TestTheRawChecksAreLoadBearing proves that claim rather than asserting it: it
	// canonicalises each one the way a lenient implementation would and shows the
	// signature verifying.
	add("duplicate_key_shadowing",
		"the forged pair is placed BEFORE the signed one, so a last-value-wins parser "+
			"reproduces the signed form exactly and the signature VERIFIES; a consumer "+
			"whose parser takes the first value reads 9 where the issuer wrote 7. "+
			"Without a raw duplicate scan this document is accepted",
		"duplicate key",
		[]byte(strings.Replace(string(signed), `"revision":7`, `"revision":9,"revision":7`, 1)))

	// The exponent sits in a field the struct does not know, so the document reaches
	// the canonical form instead of dying in a Go-specific decode error. The point is
	// the rule, and the rule lives in the canonicaliser.
	withExtra := signRawFixture(t, fixtureBody(`,"extra":7`), priv)
	add("exponent_preserving_value",
		"7e0 parses to the same 7 the issuer signed, so the signature VERIFIES over "+
			"bytes that were never produced. The rewrite is invisible to every check "+
			"made after parsing",
		"whole numbers only",
		[]byte(strings.Replace(string(withExtra), `"extra":7`, `"extra":7e0`, 1)))

	// Signed with a literal U+FFFD, then delivered with the escape that Go maps ONTO
	// U+FFFD. The canonical form is identical, so the signature holds.
	//
	// This one is asymmetric and the asymmetry is the lesson: JavaScript keeps the
	// lone surrogate, so a JS verifier refuses this document on the signature, while
	// Go — and any decoder that substitutes the replacement character — accepts it.
	// Neither implementation can tell from its own behaviour that the other is at
	// risk, which is why the check belongs in the contract rather than in whichever
	// language noticed first.
	withReplacement := signRawFixture(t, fixtureBody(`,"note":"�"`), priv)
	add("surrogate_rewrite",
		"a signed U+FFFD rewritten as \\ud800 still VERIFIES anywhere the decoder "+
			"substitutes the replacement character, Go included; the verifier would be "+
			"vouching for bytes nobody signed",
		"surrogate",
		[]byte(strings.Replace(string(withReplacement), "�", `\ud800`, 1)))

	// Codec refusals. The signature on these is a placeholder: each is refused before
	// verification reaches it, by the rule the vector is named for.
	placeholder := base64.StdEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	codecRejects := []struct{ name, why, expected, clause string }{
		{"fractional_number",
			"float spelling differs between languages (Go writes 1e+06 where JavaScript " +
				"writes 1000000), so a signed document carries whole numbers only",
			"whole numbers only", `,"ratio":1.5`},
		{"exponent_number",
			"same reason: an exponent has more than one spelling",
			"whole numbers only", `,"ratio":7e2`},
		{"non_ascii_object_key",
			"Go orders keys by byte and RFC 8785 by UTF-16 code unit; the two can " +
				"disagree above the BMP, so keys are ASCII",
			"is not ASCII", `,"ключ":1`},
		{"lone_surrogate_escape",
			"Go replaces an unpaired surrogate with U+FFFD while JavaScript keeps it; a " +
				"signature check must not quietly rewrite its own input",
			"surrogate", `,"note":"\ud800"`},
		{"integer_beyond_safe_range",
			"JavaScript's JSON.parse turns 9223372036854775807 into 9223372036854776000 " +
				"without saying so; the document would then canonicalise differently on " +
				"the two sides and the venue would be told its signature was invalid",
			"outside the range", `,"big":9223372036854775807`},
	}
	for _, c := range codecRejects {
		doc := strings.TrimSuffix(fixtureBody(c.clause), `}`) + `,"signature":"` + placeholder + `"}`
		add(c.name, c.why, c.expected, []byte(doc))
	}
	return out
}

func extractSignature(signed []byte) string {
	var p ProfileV3
	_ = json.Unmarshal(signed, &p)
	return p.Signature
}

// manifestVectors publish the key list the same way as the documents, so the next
// review is a checker run rather than a reading of this code.
func manifestVectors(t *testing.T) manifestFixtures {
	t.Helper()
	rootPriv, rootPub := fixtureRoot()
	signerPriv, signerPub := fixtureKey()
	signerB64 := base64.StdEncoding.EncodeToString(signerPub)

	var out manifestFixtures
	out.RootKey.KeyID = "fixture-root-0"
	out.RootKey.Seed = fixtureRootSeed
	out.RootKey.PublicKeyB64 = base64.StdEncoding.EncodeToString(rootPub)
	out.Environment = "production"

	sign := func(m *KeyManifest) []byte {
		raw, err := SignManifest(m, "fixture-root-0", rootPriv)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	good := func() *KeyManifest {
		return &KeyManifest{
			Issuer:      "ramio",
			Environment: "production",
			Revision:    5,
			IssuedAt:    time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC),
			Keys: []ManifestKey{{
				KeyID:                 "fixture-0",
				PublicKey:             signerB64,
				AllowedSchemaVersions: []int{SchemaVersion3},
			}},
		}
	}

	m := good()
	raw := sign(m)
	canonical, err := canonicalManifestBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	out.Valid = signedVector{
		Document:      string(raw),
		CanonicalJSON: string(canonical),
		SigningInput:  hex.EncodeToString(signingInput(signingDomainManifest, canonical)),
		Signature:     m.Signature,
	}

	add := func(name, why, stage, expected string, manifest []byte, have int64, doc string) {
		out.Negative = append(out.Negative, manifestReject{Name: name, Why: why,
			Stage: stage, ExpectedError: expected, Manifest: string(manifest),
			HaveRevision: have, Document: doc})
	}

	// Signed by the ONLINE signer instead of the offline root. This is the whole
	// reason the manifest exists.
	onlineSigned := good()
	onlineSigned.FormatVersion = 1
	onlineSigned.RootKeyID = "fixture-root-0"
	add("signed_by_the_online_signer",
		"whoever takes the running service must not be able to install a key of their "+
			"choosing; only the offline root signs the list",
		stageVerifyManifest, "invalid manifest signature",
		signManifestRaw(t, onlineSigned, signerPriv), 0, "")

	dup := good()
	dup.FormatVersion = 1
	dup.RootKeyID = "fixture-root-0"
	dup.Keys = append(dup.Keys, ManifestKey{
		KeyID: "fixture-0", PublicKey: signerB64,
		AllowedSchemaVersions: []int{SchemaVersion3}, Revoked: true})
	add("duplicate_key_id",
		"one key_id present twice, once current and once revoked: whether it survives "+
			"would depend on which entry the reader applied last",
		stageVerifyManifest, "more than once",
		signManifestRaw(t, dup, rootPriv), 0, "")

	staging := good()
	staging.Environment = "staging"
	add("wrong_environment",
		"a list built for a test rig must never apply to a venue in the field",
		stageVerifyManifest, "environment", sign(staging), 0, "")

	add("revision_does_not_advance",
		"replaying an older list is how a withdrawn key is reinstated; repeating the "+
			"CURRENT one is the ordinary case and means keep what you have",
		stageVerifyManifest, "does not advance", sign(good()), 5, "")

	allRevoked := good()
	allRevoked.Keys = []ManifestKey{{KeyID: "fixture-0", PublicKey: signerB64, Revoked: true}}
	add("all_keys_revoked",
		"a valid list saying everything is withdrawn means STOP trusting what you hold; "+
			"it must not be confused with a list that could not be read",
		stageTrusted, "every key in the manifest is revoked", sign(allRevoked), 0, "")

	// The two Cloud reproduced: constraints that were written down and then dropped.
	doc := sampleV3()
	doc.IssuedAt = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	signedDoc, err := SignV3(doc, "fixture-0", signerPriv)
	if err != nil {
		t.Fatal(err)
	}
	schemaBound := good()
	schemaBound.Keys[0].AllowedSchemaVersions = []int{2}
	add("key_not_allowed_for_this_schema",
		"the list permits this key schema 2 only; a schema 3 document signed by it "+
			"must be refused even though the signature is genuine",
		stageVerifyDocument, "not permitted to sign schema 3",
		sign(schemaBound), 0, string(signedDoc))

	closed := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	retired := good()
	retired.Keys[0].SigningUntil = &closed
	add("document_issued_after_signing_window",
		"the window bounds NEW documents and is measured against the document's own "+
			"signed issued_at, never the reader's clock",
		stageVerifyDocument, "stopped being allowed to sign",
		sign(retired), 0, string(signedDoc))

	return out
}

// signManifestRaw signs bytes the authoring checks would have refused, or with a key
// that is not the root, so the verifying side can be shown not to rely on either.
func signManifestRaw(t *testing.T, m *KeyManifest, priv ed25519.PrivateKey) []byte {
	t.Helper()
	m.Signature = ""
	unsigned, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := canonicalManifestBytes(unsigned)
	if err != nil {
		t.Fatal(err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(priv, signingInput(signingDomainManifest, canonical)))
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Every negative vector must actually be refused, AT THE NAMED STAGE and for the
// named reason. Publishing a vector the reference implementation accepts would be
// worse than publishing none; publishing one it refuses for an unrelated reason is
// how the first round of these vectors reported agreement that had not been tested.
func TestNegativeVectorsAreAllRefused(t *testing.T) {
	priv, pub := fixtureKey()
	keys := DevelopmentTrust("fixture-0", pub, SchemaVersion3)

	doc := sampleV3()
	signed, err := SignV3(doc, "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range negativeVectors(t, signed) {
		t.Run(v.Name, func(t *testing.T) {
			_, err := VerifyV3([]byte(v.Document), keys)
			if err == nil {
				t.Fatalf("vector %q was accepted; %s", v.Name, v.Why)
			}
			if !strings.Contains(err.Error(), v.ExpectedError) {
				t.Fatalf("vector %q refused for the wrong reason: got %q, want it to "+
					"mention %q", v.Name, err, v.ExpectedError)
			}
		})
	}
}

// The same for the manifest vectors, each at its own stage.
func TestManifestVectorsAreAllRefused(t *testing.T) {
	f := manifestVectors(t)
	rootPub, err := base64.StdEncoding.DecodeString(f.RootKey.PublicKeyB64)
	if err != nil {
		t.Fatal(err)
	}
	rootSet := KeySet{f.RootKey.KeyID: ed25519.PublicKey(rootPub)}

	for _, v := range f.Negative {
		t.Run(v.Name, func(t *testing.T) {
			m, err := VerifyManifest([]byte(v.Manifest), rootSet, f.Environment, v.HaveRevision)
			if v.Stage == stageVerifyManifest {
				assertRefused(t, v.Name, v.ExpectedError, err)
				return
			}
			if err != nil {
				t.Fatalf("%s: the manifest itself must verify, got %v", v.Name, err)
			}
			trusted, err := m.Trusted()
			if v.Stage == stageTrusted {
				assertRefused(t, v.Name, v.ExpectedError, err)
				return
			}
			if err != nil {
				t.Fatalf("%s: the key set must build, got %v", v.Name, err)
			}
			_, err = VerifyV3([]byte(v.Document), trusted)
			assertRefused(t, v.Name, v.ExpectedError, err)
		})
	}

	// And the valid manifest must produce a set that verifies a real document.
	m, err := VerifyManifest([]byte(f.Valid.Document), rootSet, f.Environment, 0)
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := m.Trusted()
	if err != nil {
		t.Fatal(err)
	}
	priv, _ := fixtureKey()
	signed, err := SignV3(sampleV3(), "fixture-0", priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyV3(signed, trusted); err != nil {
		t.Fatalf("the published manifest must verify a document from its own signer: %v", err)
	}
}

func assertRefused(t *testing.T, name, expected string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s was accepted", name)
	}
	if !strings.Contains(err.Error(), expected) {
		t.Fatalf("%s refused for the wrong reason: got %q, want it to mention %q",
			name, err, expected)
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
	got, err := VerifyV3(signed, DevelopmentTrust("fixture-0", pub, SchemaVersion3))
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

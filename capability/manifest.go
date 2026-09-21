package capability

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// signingDomainManifest keeps manifest signatures from being replayable as capability
// documents, and the other way round.
const signingDomainManifest = "mrps-signing-key-manifest-1"

// ErrManifestNotNewer means the manifest carries nothing this venue has not already
// applied. It is NOT a trust failure, and a consumer that treats it as one has a bug:
// Cloud repeats the current manifest on every sync, so the ordinary case is a repeat.
// The correct response is to keep the set already in force.
var ErrManifestNotNewer = errors.New("manifest revision does not advance")

// ErrAllKeysRevoked means the manifest is valid and says every key is withdrawn.
//
// It is separated from the parse and signature failures on purpose, because the two
// demand opposite responses. An unreadable manifest means keep what you have; this
// one means stop trusting what you have. A consumer that lumps them together as
// "error, keep the previous set" would go on honouring documents signed by a key
// that was revoked for compromise — which is the entire event revocation exists for.
var ErrAllKeysRevoked = errors.New("every key in the manifest is revoked")

// KeyManifest is the list of signing keys a venue should trust, signed by the OFFLINE
// ROOT rather than by the online signer.
//
// Two keys with two jobs. The online signer P issues capability documents and lives
// where the service can reach it; the offline root R only ever signs this list and
// does not live in the Cloud runtime at all. Without that split, whoever takes the
// running service can hand every venue a key of their choosing and mint any
// entitlement they like — the capability signature would still verify perfectly,
// because it would be verified against a key the attacker installed.
//
// This is why the list cannot travel as plain fields inside an authenticated sync
// response. The sync channel proves the message came from the service; it cannot
// prove the service was not the thing that was taken.
type KeyManifest struct {
	FormatVersion int    `json:"format_version"`
	Issuer        string `json:"issuer"`
	// Environment separates production from anything else, so a manifest generated
	// for a test rig cannot be applied to a venue in the field.
	Environment string `json:"environment"`
	// Revision is monotonic. A venue accepts a manifest only if it advances, which is
	// what stops an old list being replayed to reinstate a key that was withdrawn.
	Revision  int64         `json:"revision"`
	IssuedAt  time.Time     `json:"issued_at"`
	Keys      []ManifestKey `json:"keys"`
	RootKeyID string        `json:"root_key_id"`
	Signature string        `json:"signature,omitempty"`
}

type ManifestKey struct {
	KeyID string `json:"key_id"`
	// PublicKey is base64 standard encoding of the raw Ed25519 public key.
	PublicKey string `json:"public_key"`
	// AllowedSchemaVersions bounds what this key may sign. An EMPTY list permits
	// nothing: a permission that defaults to open is a permission that leaks, and a
	// manifest which forgets the field should fail loudly where it is authored rather
	// than grant everything quietly at a venue. SignManifest refuses such a key.
	AllowedSchemaVersions []int `json:"allowed_schema_versions"`
	// SigningFrom and SigningUntil bound when this key may produce NEW documents, and
	// they are compared against the document's own SIGNED issued_at — never against
	// the reader's clock.
	//
	// That is what lets a key retire without an expiry appearing where none belongs.
	// A document issued while the key was current verifies forever, which the free
	// set requires; a document claiming to be issued after the key retired is
	// refused, which is what the window is for. Using wall-clock time instead would
	// put a hidden deadline on every document that key ever signed, and would hand
	// the decision to a venue clock this project has already learned not to trust.
	SigningFrom  *time.Time `json:"signing_from,omitempty"`
	SigningUntil *time.Time `json:"signing_until,omitempty"`
	// Revoked withdraws a key entirely: it verifies nothing, not even documents
	// already issued under it. Reserved for compromise, and it is a different act
	// from retiring a key, with different consequences for the venues holding
	// documents signed by it.
	Revoked bool `json:"revoked,omitempty"`
}

// TrustedKey is a public key together with what it is permitted to sign.
// The consumer provisions trust, for example in its embedded key set or from
// a verified manifest. Unverified profile/sync fields must not create trust.
//
// The constraints travel WITH the key rather than beside it, and VerifyV3 accepts
// nothing else. Cloud's review found the earlier shape handed verification a bare map
// of public keys, so a key the manifest restricted to schema 2, and a key whose
// signing window had closed, both verified a schema 3 document issued afterwards: the
// restrictions were written down and then dropped one call later. Carrying them in
// the type is what makes dropping them impossible rather than merely discouraged.
// VerifyV2ForSubject also consumes these constraints, using the frozen v2 codec.
type TrustedKey struct {
	PublicKey             ed25519.PublicKey
	AllowedSchemaVersions []int
	SigningFrom           *time.Time
	SigningUntil          *time.Time
}

// TrustedKeys is the set VerifyV3 and VerifyV2ForSubject work against.
type TrustedKeys map[string]TrustedKey

// mayIssue reports whether this key was permitted to sign that document.
func (k TrustedKey) mayIssue(schemaVersion int, issuedAt time.Time) error {
	if len(k.PublicKey) != ed25519.PublicKeySize {
		// Checked here because ed25519.Verify PANICS on a wrong-length key — Cloud
		// reproduced it with a one-byte key. A key of the wrong size is a
		// configuration fault, and a configuration fault must come back as an error
		// the caller can report, not as a crashed process.
		return fmt.Errorf("public key is %d bytes, want %d",
			len(k.PublicKey), ed25519.PublicKeySize)
	}
	allowed := false
	for _, v := range k.AllowedSchemaVersions {
		if v == schemaVersion {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("key is not permitted to sign schema %d documents", schemaVersion)
	}
	if k.SigningFrom != nil && issuedAt.Before(*k.SigningFrom) {
		return fmt.Errorf("document is dated before this key was allowed to sign")
	}
	if k.SigningUntil != nil && issuedAt.After(*k.SigningUntil) {
		return fmt.Errorf("document is dated after this key stopped being allowed to sign")
	}
	return nil
}

// DevelopmentTrust builds a trusted set with NO manifest behind it.
//
// It exists for tests and for the development key that predates key distribution, and
// it is named so that finding every place trust is asserted without a signed manifest
// is a single grep. A key that came from a manifest must never be lifted through
// here: that would discard the very constraints the manifest carries.
func DevelopmentTrust(keyID string, pub ed25519.PublicKey, schemaVersions ...int) TrustedKeys {
	return TrustedKeys{keyID: {PublicKey: pub, AllowedSchemaVersions: schemaVersions}}
}

// SignManifest signs the list with the offline root and returns the exact bytes.
func SignManifest(m *KeyManifest, rootKeyID string, root ed25519.PrivateKey) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("nil manifest")
	}
	if rootKeyID == "" {
		return nil, fmt.Errorf("missing root_key_id")
	}
	if err := validateManifestContents(*m); err != nil {
		return nil, err
	}
	m.FormatVersion = 1
	m.RootKeyID = rootKeyID
	m.Signature = ""

	unsigned, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	canonical, err := canonicalManifestBytes(unsigned)
	if err != nil {
		return nil, err
	}
	m.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(root, signingInput(signingDomainManifest, canonical)))
	return json.Marshal(m)
}

// VerifyManifest checks a manifest against the roots this venue trusts.
//
// roots is deliberately separate from the capability key set: a venue that confused
// the two would accept a key list signed by the online signer, which is exactly the
// door this design closes.
//
// haveRevision is the revision already in force. A manifest that does not advance
// past it comes back wrapping ErrManifestNotNewer, which the caller must read as
// "nothing to do" rather than as a fault.
func VerifyManifest(raw []byte, roots KeySet, environment string, haveRevision int64) (KeyManifest, error) {
	var m KeyManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return KeyManifest{}, fmt.Errorf("parse key manifest: %w", err)
	}
	if m.FormatVersion != 1 {
		return KeyManifest{}, fmt.Errorf("unsupported manifest format_version %d", m.FormatVersion)
	}
	if m.Environment != environment {
		// A manifest built for a test rig must never apply to a venue in the field,
		// however valid its signature.
		return KeyManifest{}, fmt.Errorf("manifest is for environment %q, this venue is %q",
			m.Environment, environment)
	}
	if m.Revision <= haveRevision {
		// Replaying an older list is how a withdrawn key gets reinstated. The equal
		// case is the ordinary one — Cloud repeats the current manifest on every sync.
		return KeyManifest{}, fmt.Errorf("%w: %d does not advance past %d",
			ErrManifestNotNewer, m.Revision, haveRevision)
	}
	root, ok := roots[m.RootKeyID]
	if !ok {
		return KeyManifest{}, fmt.Errorf("unknown root_key_id %q", m.RootKeyID)
	}
	if len(root) != ed25519.PublicKeySize {
		return KeyManifest{}, fmt.Errorf("trusted root %q is %d bytes, want %d",
			m.RootKeyID, len(root), ed25519.PublicKeySize)
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return KeyManifest{}, fmt.Errorf("decode manifest signature: %w", err)
	}
	canonical, err := canonicalManifestBytes(raw)
	if err != nil {
		return KeyManifest{}, err
	}
	if !ed25519.Verify(root, signingInput(signingDomainManifest, canonical), sig) {
		return KeyManifest{}, fmt.Errorf("invalid manifest signature")
	}
	if err := validateManifestContents(m); err != nil {
		return KeyManifest{}, err
	}
	return m, nil
}

// validateManifestContents holds the rules a manifest must satisfy whoever produced
// it, checked both before signing and after verifying.
func validateManifestContents(m KeyManifest) error {
	if m.Issuer == "" {
		return fmt.Errorf("manifest has no issuer")
	}
	if m.Environment == "" {
		return fmt.Errorf("manifest has no environment")
	}
	if m.Revision <= 0 {
		return fmt.Errorf("manifest revision must be positive")
	}
	if len(m.Keys) == 0 {
		// An empty list would leave a venue unable to verify anything at all, which
		// is an outage rather than a policy. Withdrawing a key is done by marking it
		// revoked, which says so.
		return fmt.Errorf("manifest carries no keys")
	}
	seen := map[string]bool{}
	for _, k := range m.Keys {
		if k.KeyID == "" {
			return fmt.Errorf("manifest has a key with no key_id")
		}
		if seen[k.KeyID] {
			// Cloud's review: with one key_id present twice, once current and once
			// revoked, whether the key ends up trusted depends on which entry the
			// reader applies last. Revocation that can be undone by appending a
			// duplicate is not revocation. A list with two answers for one key has no
			// answer, so the whole manifest is refused.
			return fmt.Errorf("key_id %q appears more than once; revocation would be ambiguous", k.KeyID)
		}
		seen[k.KeyID] = true
		raw, err := base64.StdEncoding.DecodeString(k.PublicKey)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return fmt.Errorf("key %q has an unusable public key", k.KeyID)
		}
		if len(k.AllowedSchemaVersions) == 0 && !k.Revoked {
			return fmt.Errorf("key %q lists no allowed schema versions, so it could sign nothing", k.KeyID)
		}
		if k.SigningFrom != nil && k.SigningUntil != nil && !k.SigningUntil.After(*k.SigningFrom) {
			return fmt.Errorf("key %q has a signing window that ends before it starts", k.KeyID)
		}
	}
	return nil
}

// Trusted turns a verified manifest into the set used to check capability documents,
// constraints included.
//
// Revoked keys are left out; retired ones are kept, because they still have to verify
// what they signed while current — their SigningUntil rides along and is enforced
// against each document's signed issued_at.
func (m KeyManifest) Trusted() (TrustedKeys, error) {
	out := TrustedKeys{}
	for _, k := range m.Keys {
		if k.Revoked {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(k.PublicKey)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("key %q has an unusable public key", k.KeyID)
		}
		out[k.KeyID] = TrustedKey{
			PublicKey:             ed25519.PublicKey(raw),
			AllowedSchemaVersions: append([]int(nil), k.AllowedSchemaVersions...),
			SigningFrom:           k.SigningFrom,
			SigningUntil:          k.SigningUntil,
		}
	}
	if len(out) == 0 {
		return nil, ErrAllKeysRevoked
	}
	return out, nil
}

func canonicalManifestBytes(raw []byte) ([]byte, error) {
	return canonicalWithout(raw, "signature")
}

func signingInput(domain string, canonical []byte) []byte {
	out := make([]byte, 0, len(domain)+1+len(canonical))
	out = append(out, domain...)
	out = append(out, 0)
	return append(out, canonical...)
}

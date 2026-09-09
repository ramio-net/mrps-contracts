package capability

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// signingDomainManifest keeps manifest signatures from being replayable as capability
// documents, and the other way round.
const signingDomainManifest = "mrps-signing-key-manifest-1"

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
	// AllowedSchemaVersions bounds what this key may sign, so a key issued for one
	// document format cannot be used to sign another.
	AllowedSchemaVersions []int `json:"allowed_schema_versions,omitempty"`
	// SigningFrom and SigningUntil bound when this key may produce NEW documents.
	// They do not bound verification: a key past SigningUntil still verifies
	// everything it signed while it was current, which is the point — dropping it
	// early would put a hidden expiry on free sets that are supposed to have none.
	SigningFrom  *time.Time `json:"signing_from,omitempty"`
	SigningUntil *time.Time `json:"signing_until,omitempty"`
	// Revoked withdraws a key entirely: it verifies nothing, not even documents
	// already issued under it. Reserved for compromise, and it is a different act
	// from retiring a key, with different consequences for the venues holding
	// documents signed by it.
	Revoked bool `json:"revoked,omitempty"`
}

// SignManifest signs the list with the offline root and returns the exact bytes.
func SignManifest(m *KeyManifest, rootKeyID string, root ed25519.PrivateKey) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("nil manifest")
	}
	if rootKeyID == "" {
		return nil, fmt.Errorf("missing root_key_id")
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
// roots is deliberately separate from the capability KeySet: a venue that confuses
// the two would accept a key list signed by the online signer, which is exactly the
// door this design closes.
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
		// Replaying an older list is how a withdrawn key gets reinstated.
		return KeyManifest{}, fmt.Errorf("manifest revision %d does not advance past %d",
			m.Revision, haveRevision)
	}
	root, ok := roots[m.RootKeyID]
	if !ok {
		return KeyManifest{}, fmt.Errorf("unknown root_key_id %q", m.RootKeyID)
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
	if len(m.Keys) == 0 {
		// An empty list would leave a venue unable to verify anything at all, which
		// is an outage rather than a policy.
		return KeyManifest{}, fmt.Errorf("manifest carries no keys")
	}
	return m, nil
}

// VerifyKeySet turns a verified manifest into the set used to check capability
// documents. Revoked keys are left out; retired ones are kept, because they still
// have to verify what they signed while current.
func (m KeyManifest) VerifyKeySet() (KeySet, error) {
	out := KeySet{}
	for _, k := range m.Keys {
		if k.Revoked {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(k.PublicKey)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("key %q has an unusable public key", k.KeyID)
		}
		out[k.KeyID] = ed25519.PublicKey(raw)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("every key in the manifest is revoked")
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

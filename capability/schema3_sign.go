package capability

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// SignV3 signs a document and returns the exact bytes that were signed.
//
// The bytes are returned rather than only stored on the struct because they are what
// a verifier must see: re-marshalling the struct later can differ, and the whole
// point of schema 3 is that verification works on what arrived, not on what this
// build would have produced.
func SignV3(p *ProfileV3, keyID string, priv ed25519.PrivateKey) ([]byte, error) {
	if p == nil {
		return nil, fmt.Errorf("nil profile")
	}
	if keyID == "" {
		return nil, fmt.Errorf("missing key_id")
	}
	p.SchemaVersion = SchemaVersion3
	p.KeyID = keyID
	p.Signature = ""

	unsigned, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	canonical, err := CanonicalBytesV3(unsigned)
	if err != nil {
		return nil, err
	}
	p.Signature = base64.StdEncoding.EncodeToString(
		ed25519.Sign(priv, SigningInputV3(canonical)))
	return json.Marshal(p)
}

// VerifyV3 checks a received document against a SET of trusted keys.
//
// A set rather than one key, because rotation without overlap is an outage: the new
// key has to be trusted before it is used, and the old one has to stay trusted after
// it stops being used. Removing a retired key is a separate, deliberate act — keep
// it verify-only, or documents already issued under it stop being readable, which
// would put a hidden expiry on a free set that is supposed to have none.
//
// The set is TrustedKeys rather than a bare map of public keys so that what the
// manifest permits each key to sign arrives here attached to the key. There is no
// second call to forget.
func VerifyV3(raw []byte, keys TrustedKeys) (ProfileV3, error) {
	var p ProfileV3
	// Decoded with the same whole-document rule the canonicaliser applies, so that a
	// document with something after it is refused HERE and for a reason both
	// implementations can print. Left to encoding/json it is still refused, but with
	// a Go-specific message no other language would produce, and a published vector
	// whose expected reason is one library's wording is not a contract.
	if err := decodeWholeDocument(raw, &p); err != nil {
		return ProfileV3{}, err
	}
	if p.SchemaVersion != SchemaVersion3 {
		// Named, so the diagnosis is "this build cannot read that schema" and not
		// "the signature is bad". They have different cures: one is an update here,
		// the other is a look at Cloud and at keys.
		return ProfileV3{}, fmt.Errorf("unsupported schema_version %d", p.SchemaVersion)
	}
	if p.Signature == "" {
		return ProfileV3{}, fmt.Errorf("missing signature")
	}
	if p.KeyID == "" {
		return ProfileV3{}, fmt.Errorf("missing key_id")
	}
	key, ok := keys[p.KeyID]
	if !ok {
		return ProfileV3{}, fmt.Errorf("unknown key_id %q", p.KeyID)
	}
	// What the key was permitted to sign, before asking whether it did sign this.
	// Checked against the document's own signed issued_at, so the answer does not
	// depend on the reader's clock and does not change as time passes.
	if err := key.mayIssue(p.SchemaVersion, p.IssuedAt); err != nil {
		return ProfileV3{}, fmt.Errorf("key %q: %w", p.KeyID, err)
	}
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	if err != nil {
		return ProfileV3{}, fmt.Errorf("decode signature: %w", err)
	}
	canonical, err := CanonicalBytesV3(raw)
	if err != nil {
		return ProfileV3{}, err
	}
	if !ed25519.Verify(key.PublicKey, SigningInputV3(canonical), sig) {
		return ProfileV3{}, fmt.Errorf("invalid signature")
	}
	// A document whose free set is missing cannot be accepted at all: the step down
	// after the paid right ends would have nowhere to land, and the venue would fall
	// to whatever floor the binary happens to carry.
	if p.Registered.Limits.MaxCameras <= 0 {
		return ProfileV3{}, fmt.Errorf("registered set is missing or unusable")
	}
	if p.OfflineWindowSec <= 0 {
		return ProfileV3{}, fmt.Errorf("offline_window_sec must be positive")
	}
	return p, nil
}

// VerifyV3ForSubject adds the check that the document was issued to THIS venue.
func VerifyV3ForSubject(raw []byte, keys TrustedKeys, expected SubjectV3) (ProfileV3, error) {
	p, err := VerifyV3(raw, keys)
	if err != nil {
		return ProfileV3{}, err
	}
	switch {
	case p.Subject.InstallationID != expected.InstallationID:
		return ProfileV3{}, fmt.Errorf("subject installation_id mismatch")
	case p.Subject.EdgeID != expected.EdgeID:
		return ProfileV3{}, fmt.Errorf("subject edge_id mismatch")
	case p.Subject.OrgID != expected.OrgID:
		return ProfileV3{}, fmt.Errorf("subject org_id mismatch")
	}
	return p, nil
}

// PaidUntil reports the deadline of the paid set: the earlier of what was bought and
// how long the venue may apply it without reaching Cloud.
//
// Both halves matter and they answer different questions. valid_until is what was
// paid for; issued_at + offline_window is how stale an assertion about payment may
// get before it stops being trusted. A subscription with thirty days left still
// steps down if the venue has not been heard from for a week.
//
// ok is false when nothing is paid for, which is not an error — it is the free tier.
func (p ProfileV3) PaidUntil() (deadline time.Time, ok bool) {
	if p.Full == nil {
		return time.Time{}, false
	}
	offline := p.IssuedAt.Add(time.Duration(p.OfflineWindowSec) * time.Second)
	if p.Full.ValidUntil.Before(offline) {
		return p.Full.ValidUntil, true
	}
	return offline, true
}

// PaidActiveAt reports whether the paid set applies at this moment. Equality with a
// deadline counts as expired.
func (p ProfileV3) PaidActiveAt(now time.Time) bool {
	deadline, ok := p.PaidUntil()
	if !ok {
		return false
	}
	if !p.Full.ValidFrom.IsZero() && now.Before(p.Full.ValidFrom) {
		return false
	}
	return now.Before(deadline)
}

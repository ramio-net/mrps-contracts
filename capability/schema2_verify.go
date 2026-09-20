package capability

import "fmt"

// VerifyV2ForSubject verifies a schema-2 profile without dropping the trusted
// key's schema permissions or signing window. The window applies to the signed
// issued_at, not the reader's clock; entitlement expiry is separate policy.
// On error, callers must not apply the profile or retry with the bare KeySet API.
// This function never discovers keys, accepts unsigned demo, or adds dev trust.
func VerifyV2ForSubject(p Profile, keys TrustedKeys, expected Subject) error {
	if p.SchemaVersion != 2 {
		return fmt.Errorf("unsupported schema_version %d", p.SchemaVersion)
	}
	if p.Signature == "" {
		return fmt.Errorf("missing signature")
	}
	if p.KeyID == "" {
		return fmt.Errorf("missing key_id")
	}
	if p.IssuedAt.IsZero() {
		return fmt.Errorf("missing issued_at")
	}
	key, ok := keys[p.KeyID]
	if !ok {
		return fmt.Errorf("unknown key_id %q", p.KeyID)
	}
	// Use the same constraints as schema 3, including the length guard that
	// prevents ed25519.Verify from panicking on malformed public material.
	if err := key.mayIssue(p.SchemaVersion, p.IssuedAt); err != nil {
		return fmt.Errorf("key %q: %w", p.KeyID, err)
	}
	return VerifyForSubject(p, KeySet{p.KeyID: key.PublicKey}, expected)
}

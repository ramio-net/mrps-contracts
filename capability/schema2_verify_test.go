package capability

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

type schema2WindowCase struct {
	Profile  Profile
	Keys     TrustedKeys
	Expected Subject
}

func (c *schema2WindowCase) changeKey(change func(*TrustedKey)) {
	key := c.Keys["test-window-key"]
	change(&key)
	c.Keys["test-window-key"] = key
}

func TestVerifyV2ForSubjectVectors(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately historical: retirement must not depend on the reader's clock.
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for _, v := range []struct {
		name, reason, wantError string
		change, tamper          func(*schema2WindowCase)
		legacyAccept            bool
	}{
		{name: "historic_retired", reason: "within_signing_window", legacyAccept: true},
		{name: "at_start", reason: "inclusive_start", change: func(c *schema2WindowCase) { c.Profile.IssuedAt = start }, legacyAccept: true},
		{name: "at_end", reason: "inclusive_end", change: func(c *schema2WindowCase) { c.Profile.IssuedAt = end }, legacyAccept: true},
		{name: "offset_at_end", reason: "same_instant", change: func(c *schema2WindowCase) { c.Profile.IssuedAt = end.In(time.FixedZone("UTC+3", 3*60*60)) }, legacyAccept: true},
		{name: "no_window", reason: "schema_permitted", change: func(c *schema2WindowCase) {
			c.changeKey(func(k *TrustedKey) { k.SigningFrom, k.SigningUntil = nil, nil })
		}, legacyAccept: true},
		{name: "only_from", reason: "no_upper_bound", change: func(c *schema2WindowCase) {
			c.changeKey(func(k *TrustedKey) { k.SigningUntil = nil })
			c.Profile.IssuedAt = end.Add(time.Hour)
		}, legacyAccept: true},
		{name: "only_until", reason: "no_lower_bound", change: func(c *schema2WindowCase) {
			c.changeKey(func(k *TrustedKey) { k.SigningFrom = nil })
			c.Profile.IssuedAt = start.Add(-time.Hour)
		}, legacyAccept: true},
		{name: "before_start", reason: "before_signing_window", wantError: "document is dated before this key was allowed to sign", change: func(c *schema2WindowCase) { c.Profile.IssuedAt = start.Add(-time.Nanosecond) }, legacyAccept: true},
		{name: "after_end", reason: "after_signing_window", wantError: "document is dated after this key stopped being allowed to sign", change: func(c *schema2WindowCase) { c.Profile.IssuedAt = end.Add(time.Nanosecond) }, legacyAccept: true},
		{name: "empty_permissions", reason: "schema_not_permitted", wantError: "not permitted to sign schema 2", change: func(c *schema2WindowCase) { c.changeKey(func(k *TrustedKey) { k.AllowedSchemaVersions = nil }) }, legacyAccept: true},
		{name: "schema3_only", reason: "schema_not_permitted", wantError: "not permitted to sign schema 2", change: func(c *schema2WindowCase) { c.changeKey(func(k *TrustedKey) { k.AllowedSchemaVersions = []int{3} }) }, legacyAccept: true},
		{name: "unknown_key", reason: "unknown_key_id", wantError: "unknown key_id", change: func(c *schema2WindowCase) { c.Profile.KeyID = "uninstalled" }},
		{name: "missing_key_id", reason: "missing_key_id", wantError: "missing key_id", change: func(c *schema2WindowCase) { c.Profile.KeyID = "" }},
		{name: "nil_trust", reason: "unknown_key_id", wantError: "unknown key_id", change: func(c *schema2WindowCase) { c.Keys = nil }},
		{name: "empty_trust", reason: "unknown_key_id", wantError: "unknown key_id", change: func(c *schema2WindowCase) { c.Keys = TrustedKeys{} }},
		{name: "nil_public", reason: "invalid_public_key_length", wantError: "public key is 0 bytes, want 32", change: func(c *schema2WindowCase) { c.changeKey(func(k *TrustedKey) { k.PublicKey = nil }) }},
		{name: "short_public", reason: "invalid_public_key_length", wantError: "public key is 31 bytes, want 32", change: func(c *schema2WindowCase) { c.changeKey(func(k *TrustedKey) { k.PublicKey = public[:31] }) }},
		{name: "long_public", reason: "invalid_public_key_length", wantError: "public key is 33 bytes, want 32", change: func(c *schema2WindowCase) { c.changeKey(func(k *TrustedKey) { k.PublicKey = make([]byte, 33) }) }},
		{name: "wrong_material", reason: "invalid_signature", wantError: "invalid signature", change: func(c *schema2WindowCase) { c.changeKey(func(k *TrustedKey) { k.PublicKey = other }) }},
		{name: "zero_issued_at", reason: "missing_issued_at", wantError: "missing issued_at", change: func(c *schema2WindowCase) { c.Profile.IssuedAt = time.Time{} }, legacyAccept: true},
		{name: "missing_schema", reason: "unsupported_schema", wantError: "unsupported schema_version 0", change: func(c *schema2WindowCase) { c.Profile.SchemaVersion = 0 }, legacyAccept: true},
		{name: "schema3_profile", reason: "unsupported_schema", wantError: "unsupported schema_version 3", change: func(c *schema2WindowCase) { c.Profile.SchemaVersion = 3 }, legacyAccept: true},
		{name: "missing_signature", reason: "missing_signature", wantError: "missing signature", tamper: func(c *schema2WindowCase) { c.Profile.Signature = "" }},
		{name: "bad_base64", reason: "malformed_signature", wantError: "decode signature:", tamper: func(c *schema2WindowCase) { c.Profile.Signature = "%%%" }},
		{name: "short_signature", reason: "invalid_signature", wantError: "invalid signature", tamper: func(c *schema2WindowCase) { c.Profile.Signature = base64.StdEncoding.EncodeToString(make([]byte, 31)) }},
		{name: "tampered_issued_at", reason: "invalid_signature", wantError: "invalid signature", tamper: func(c *schema2WindowCase) { c.Profile.IssuedAt = c.Profile.IssuedAt.Add(time.Second) }},
		{name: "missing_subject", reason: "missing_subject", wantError: "missing subject", change: func(c *schema2WindowCase) { c.Profile.Subject = nil }},
		{name: "foreign_installation", reason: "installation_mismatch", wantError: "subject installation_id mismatch", change: func(c *schema2WindowCase) { c.Expected.InstallationID = "another-installation" }},
		{name: "foreign_edge", reason: "edge_mismatch", wantError: "subject edge_id mismatch", change: func(c *schema2WindowCase) { c.Expected.EdgeID = "another-edge" }},
		{name: "foreign_org", reason: "org_mismatch", wantError: "subject org_id mismatch", change: func(c *schema2WindowCase) { c.Expected.OrgID = "another-org" }},
	} {
		t.Run(v.name, func(t *testing.T) {
			from, until := start, end
			c := schema2WindowCase{
				Profile:  RegisteredPlatformTemplate(start.Add(30*time.Minute), testSubject()),
				Keys:     TrustedKeys{"test-window-key": {PublicKey: append(ed25519.PublicKey(nil), public...), AllowedSchemaVersions: []int{2, 3}, SigningFrom: &from, SigningUntil: &until}},
				Expected: testSubject(),
			}
			c.Profile.KeyID = "test-window-key"
			if v.change != nil {
				v.change(&c)
			}
			// Sign changed dates/subjects honestly, so constraint vectors cannot pass
			// merely because they accidentally broke the cryptographic signature.
			if err := Sign(&c.Profile, c.Profile.KeyID, private); err != nil {
				t.Fatal(err)
			}
			if v.tamper != nil {
				v.tamper(&c)
			}
			before, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			err = VerifyV2ForSubject(c.Profile, c.Keys, c.Expected)
			if v.wantError == "" {
				if err != nil {
					t.Fatalf("stage=verify_profile reason=%s unexpectedly refused: %v", v.reason, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), v.wantError) {
				t.Fatalf("stage=verify_profile reason=%s want=%q got=%v", v.reason, v.wantError, err)
			}
			after, marshalErr := json.Marshal(c)
			if marshalErr != nil || !bytes.Equal(before, after) {
				t.Fatal("verification mutated inputs", marshalErr)
			}
			if v.legacyAccept {
				if err := VerifyForSubject(c.Profile, KeySet{c.Profile.KeyID: public}, c.Expected); err != nil {
					t.Fatal("legacy positive control failed; cannot prove constraint enforcement", err)
				}
			}
			t.Logf("stage=verify_profile reason=%s accepted=%t", v.reason, err == nil)
		})
	}
}

func TestVerifyV2ForSubjectHistoricFixtureAndWithdrawal(t *testing.T) {
	raw, err := os.ReadFile("testdata/schema2_v060_profile.json")
	if err != nil {
		t.Fatal(err)
	}
	var p Profile
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.Subject == nil || !strings.Contains(p.KeyID, "&") {
		t.Fatal("historical cross-version fixture lost its subject/escaping control")
	}
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)) // Existing test fixture only.
	public := private.Public().(ed25519.PublicKey)
	from, until := p.IssuedAt.Add(-time.Hour), p.IssuedAt.Add(time.Hour)
	keys := TrustedKeys{p.KeyID: {PublicKey: public, AllowedSchemaVersions: []int{2, 3}, SigningFrom: &from, SigningUntil: &until}}
	if err := VerifyV2ForSubject(p, keys, *p.Subject); err != nil {
		t.Fatal("unchanged v0.6.0 signature failed constrained verification", err)
	}
	if err := VerifyForSubject(p, KeySet{p.KeyID: public}, *p.Subject); err != nil {
		t.Fatal("legacy API changed", err)
	}
	signedAgain := p
	if err := Sign(&signedAgain, p.KeyID, private); err != nil || signedAgain.Signature != p.Signature {
		t.Fatal("historical signing bytes changed", err)
	}
	withdrawn := TrustedKeys{}
	if err := VerifyV2ForSubject(p, withdrawn, *p.Subject); err == nil || !strings.Contains(err.Error(), "unknown key_id") {
		t.Fatal("withdrawn key still verified, or refusal reason changed", err)
	}
	if err := VerifyV2ForSubject(p, keys, *p.Subject); err != nil {
		t.Fatal("old embedded set residual-risk control failed", err)
	}
	t.Log("stage=verify_profile reason=historical_signature_unchanged accepted=true")
	t.Log("stage=verify_profile reason=withdrawn_key accepted=false; old embedded set still accepts")
}

func TestVerifyV2ForSubjectHasNoDevOrDemoFallback(t *testing.T) {
	p := RegisteredPlatformTemplate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), testSubject())
	if err := Sign(&p, DevKeyID, DevPrivateKey()); err != nil {
		t.Fatal(err)
	}
	if err := VerifyV2ForSubject(p, nil, testSubject()); err == nil || !strings.Contains(err.Error(), "unknown key_id") {
		t.Fatal("empty trust implicitly admitted dev", err)
	}
	// Explicit development trust remains possible for tests, never automatic.
	if err := VerifyV2ForSubject(p, DevelopmentTrust(DevKeyID, DevPublicKey(), 2), testSubject()); err != nil {
		t.Fatal("explicit test trust failed", err)
	}
	demo := DemoProfile(p.IssuedAt)
	if err := VerifyV2ForSubject(demo, nil, testSubject()); err == nil || !strings.Contains(err.Error(), "missing signature") {
		t.Fatal("unsigned demo bypassed the verifier", err)
	}
	t.Log("stage=verify_profile reason=no_implicit_dev_or_demo_trust accepted=false")
}

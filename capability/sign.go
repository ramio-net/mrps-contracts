package capability

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

type KeySet map[string]ed25519.PublicKey

const DevKeyID = "dev"

func DevPrivateKey() ed25519.PrivateKey {
	seed := sha256.Sum256([]byte("mrps-edge-dev-signing-v1"))
	return ed25519.NewKeyFromSeed(seed[:])
}

func DevPublicKey() ed25519.PublicKey {
	return DevPrivateKey().Public().(ed25519.PublicKey)
}

func DevKeySet() KeySet {
	return KeySet{DevKeyID: DevPublicKey()}
}

func Sign(p *Profile, keyID string, priv ed25519.PrivateKey) error {
	if p == nil {
		return fmt.Errorf("nil profile")
	}
	p.KeyID = keyID
	p.Signature = ""
	payload, err := CanonicalBytes(*p)
	if err != nil {
		return err
	}
	p.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload))
	return nil
}

func Verify(p Profile, keys KeySet) error {
	if p.Signature == "" {
		return fmt.Errorf("missing signature")
	}
	if p.KeyID == "" {
		return fmt.Errorf("missing key_id")
	}
	pub, ok := keys[p.KeyID]
	if !ok {
		return fmt.Errorf("unknown key_id %q", p.KeyID)
	}
	sig, err := base64.StdEncoding.DecodeString(p.Signature)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	payload, err := CanonicalBytes(p)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, payload, sig) {
		return fmt.Errorf("invalid signature")
	}
	return nil
}

func VerifyForSubject(p Profile, keys KeySet, expected Subject) error {
	if err := Verify(p, keys); err != nil {
		return err
	}
	if p.Subject == nil {
		return fmt.Errorf("missing subject")
	}
	if p.Subject.InstallationID != expected.InstallationID {
		return fmt.Errorf("subject installation_id mismatch")
	}
	if p.Subject.EdgeID != expected.EdgeID {
		return fmt.Errorf("subject edge_id mismatch")
	}
	if p.Subject.OrgID != expected.OrgID {
		return fmt.Errorf("subject org_id mismatch")
	}
	return nil
}

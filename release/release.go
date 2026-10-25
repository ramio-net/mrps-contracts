// Package release is the signature on a published Ramio file (a release zip, the installer): what is
// signed, how the signature travels, how it is checked.
//
// Why it exists: Ramio Edge has no Authenticode certificate (none is sold to a Russian company today),
// so Windows cannot tell a genuine Edge from a substitute. The web installer and Edge's update button
// download releases by themselves; without a signature of our own, whoever controlled the download
// server — or the road to it — could hand every venue a program of their choosing through the update
// button. The private key therefore never sits on the server: the owner signs on the build laptop when a
// release kit is prepared, and the server only serves what was signed (decision of 25.10.2026).
//
// What is signed is not the file itself but a short text that names it:
//
//	ramio-release-v1\n<product>\n<version>\n<sha256 of the file, lowercase hex>\n
//
// so a signature for one product or version cannot be passed off as another's, and a verifier needs only
// the file's digest, never a second copy of the file. The signature is Ed25519 over those bytes, carried
// as standard base64 with padding (RFC 4648 §4): in latest.json as "signature", and in a sidecar file
// "<file>.sig" holding exactly that text and one "\n".
//
// Every field is checked for its one canonical spelling before anything is signed or verified. The message
// must be byte-identical on both sides, and a verifier that tolerated "ABC…" for "abc…" or a stray space
// in a version would accept, or reject, things the signer never meant.
package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Format is the first line of every signed message. A new layout gets a new name, never a reinterpretation.
const Format = "ramio-release-v1"

// Products signed today. The set is open on purpose: a new product is a new name, and a verifier pins the
// product it expects rather than trusting whatever a manifest says.
const (
	ProductEdge          = "ramio-edge"           // the Edge release zip
	ProductEdgeInstaller = "ramio-edge-installer" // RamioEdgeSetup.exe
)

var (
	productRe = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)
	// versionRe admits what latest.json carries today ("0.7.0+12999d2", "1.0.0") and nothing that could
	// change the message's layout: no spaces, no line breaks.
	versionRe = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z.+-]{0,63}$`)
	sha256Re  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Errors a verifier can tell apart. ErrBadSignature is the one that means "not ours": the inputs were well
// formed and no trusted key produced this signature.
var (
	ErrBadProduct    = errors.New("release: product is not a lowercase name")
	ErrBadVersion    = errors.New("release: version has characters outside [0-9A-Za-z.+-] or is too long")
	ErrBadDigest     = errors.New("release: sha256 is not 64 lowercase hex characters")
	ErrBadEncoding   = errors.New("release: signature is not standard base64 of 64 bytes")
	ErrNoTrustedKeys = errors.New("release: no trusted public key to verify with")
	ErrBadSignature  = errors.New("release: signature does not match any trusted key")
)

// Message returns the exact bytes that are signed for a file: the format line, the product, the version and
// the file's SHA-256 in lowercase hex, each followed by "\n".
func Message(product, version, sha256Hex string) ([]byte, error) {
	if !productRe.MatchString(product) {
		return nil, ErrBadProduct
	}
	if !versionRe.MatchString(version) {
		return nil, ErrBadVersion
	}
	if !sha256Re.MatchString(sha256Hex) {
		return nil, ErrBadDigest
	}
	return []byte(Format + "\n" + product + "\n" + version + "\n" + sha256Hex + "\n"), nil
}

// Sign returns the base64 signature for a file. Used by the release-signing tool on the build laptop and by
// tests; never on a server.
func Sign(priv ed25519.PrivateKey, product, version, sha256Hex string) (string, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return "", errors.New("release: private key has the wrong size")
	}
	msg, err := Message(product, version, sha256Hex)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg)), nil
}

// Verify checks a signature against every trusted key and succeeds when one of them made it. More than one
// key is trusted only while a key is being replaced: a build that trusts the old and the new key ships first,
// then releases are signed with the new one — the same order as for the capability key p0.
func Verify(trusted []ed25519.PublicKey, product, version, sha256Hex, signature string) error {
	msg, err := Message(product, version, sha256Hex)
	if err != nil {
		return err
	}
	sig, err := decodeSignature(signature)
	if err != nil {
		return err
	}
	usable := 0
	for _, pub := range trusted {
		if len(pub) != ed25519.PublicKeySize {
			continue
		}
		usable++
		if ed25519.Verify(pub, msg, sig) {
			return nil
		}
	}
	if usable == 0 {
		return ErrNoTrustedKeys
	}
	return ErrBadSignature
}

// decodeSignature accepts only the canonical form: standard base64 with padding, nothing around it, 64 bytes
// once decoded. The sidecar file's single trailing "\n" is removed by SidecarSignature, not here.
func decodeSignature(s string) ([]byte, error) {
	if strings.TrimSpace(s) != s || s == "" {
		return nil, ErrBadEncoding
	}
	sig, err := base64.StdEncoding.Strict().DecodeString(s)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, ErrBadEncoding
	}
	return sig, nil
}

// SidecarContent is what goes into "<file>.sig": the signature and one line break.
func SidecarContent(signature string) string { return signature + "\n" }

// SidecarSignature reads the signature back out of a sidecar file's content. Exactly one trailing "\n" is
// allowed; anything else is malformed rather than silently trimmed.
func SidecarSignature(content string) (string, error) {
	if !strings.HasSuffix(content, "\n") {
		return "", ErrBadEncoding
	}
	s := strings.TrimSuffix(content, "\n")
	if _, err := decodeSignature(s); err != nil {
		return "", err
	}
	return s, nil
}

// FileSHA256 returns the lowercase hex SHA-256 of everything read from r — the digest the message names.
func FileSHA256(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("release: reading the file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// KeyID names a public key in logs and in latest.json ("signature_key"): the first 8 bytes of its SHA-256 in
// hex. It is a label for people, not a security check — Verify always tries every trusted key.
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

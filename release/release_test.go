package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

type vectorFile struct {
	Format         string `json:"format"`
	TestSeedHex    string `json:"test_seed_hex"`
	PublicKeyB64   string `json:"public_key_b64"`
	KeyID          string `json:"key_id"`
	SidecarExample string `json:"sidecar_example"`
	Cases          []struct {
		Name      string `json:"name"`
		Product   string `json:"product"`
		Version   string `json:"version"`
		SHA256    string `json:"sha256"`
		Message   string `json:"message"`
		Signature string `json:"signature"`
		Valid     bool   `json:"valid"`
		Error     string `json:"error"`
	} `json:"cases"`
}

var errByName = map[string]error{
	"bad_product":   ErrBadProduct,
	"bad_version":   ErrBadVersion,
	"bad_digest":    ErrBadDigest,
	"bad_encoding":  ErrBadEncoding,
	"bad_signature": ErrBadSignature,
}

func loadVectors(t *testing.T) (vectorFile, ed25519.PrivateKey, ed25519.PublicKey) {
	t.Helper()
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v vectorFile
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	seed, err := hex.DecodeString(v.TestSeedHex)
	if err != nil {
		t.Fatal(err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	if got := base64.StdEncoding.EncodeToString(pub); got != v.PublicKeyB64 {
		t.Fatalf("vector public key %s does not belong to the vector seed (%s)", v.PublicKeyB64, got)
	}
	return v, priv, pub
}

// The vectors are what Edge, the installer and Cloud's checks are all held to: every case verifies or fails
// here exactly as the file says, with the same reason.
func TestVectors(t *testing.T) {
	v, _, pub := loadVectors(t)
	if v.Format != Format {
		t.Fatalf("vectors are for %q, package is %q", v.Format, Format)
	}
	if v.KeyID != KeyID(pub) {
		t.Fatalf("key_id %s, KeyID gives %s", v.KeyID, KeyID(pub))
	}
	if len(v.Cases) == 0 {
		t.Fatal("no cases")
	}
	for _, c := range v.Cases {
		t.Run(c.Name, func(t *testing.T) {
			if c.Message != "" {
				msg, err := Message(c.Product, c.Version, c.SHA256)
				if err != nil || string(msg) != c.Message {
					t.Fatalf("message %q, want %q (err %v)", msg, c.Message, err)
				}
			}
			err := Verify([]ed25519.PublicKey{pub}, c.Product, c.Version, c.SHA256, c.Signature)
			if c.Valid {
				if err != nil {
					t.Fatalf("valid case failed: %v", err)
				}
				return
			}
			want, ok := errByName[c.Error]
			if !ok {
				t.Fatalf("unknown expected error %q", c.Error)
			}
			if !errors.Is(err, want) {
				t.Fatalf("got %v, want %v", err, want)
			}
		})
	}
}

// Ed25519 is deterministic: the same key and message give the same signature, so the vector signatures must
// be reproducible from the vector seed. A generator that drifted would show here.
func TestVectorSignaturesAreReproducible(t *testing.T) {
	v, priv, _ := loadVectors(t)
	for _, c := range v.Cases {
		if !c.Valid {
			continue
		}
		got, err := Sign(priv, c.Product, c.Version, c.SHA256)
		if err != nil || got != c.Signature {
			t.Fatalf("%s: Sign gives %s, vector has %s (err %v)", c.Name, got, c.Signature, err)
		}
	}
	if got, err := SidecarSignature(v.SidecarExample); err != nil || got != v.Cases[0].Signature {
		t.Fatalf("sidecar example reads back as %q (err %v)", got, err)
	}
}

// Replacing the key: a build trusting both the old and the new key accepts releases signed with either; one
// trusting only the new key refuses the old ones.
func TestKeyRotation(t *testing.T) {
	oldPriv := ed25519.NewKeyFromSeed(make([]byte, 32))
	newSeed := sha256.Sum256([]byte("new"))
	newPriv := ed25519.NewKeyFromSeed(newSeed[:])
	oldPub, newPub := oldPriv.Public().(ed25519.PublicKey), newPriv.Public().(ed25519.PublicKey)
	digest := strings.Repeat("a", 64)

	byOld, _ := Sign(oldPriv, ProductEdge, "1.0.0", digest)
	byNew, _ := Sign(newPriv, ProductEdge, "1.0.1", digest)
	both := []ed25519.PublicKey{oldPub, newPub}
	if err := Verify(both, ProductEdge, "1.0.0", digest, byOld); err != nil {
		t.Fatalf("old key during rotation: %v", err)
	}
	if err := Verify(both, ProductEdge, "1.0.1", digest, byNew); err != nil {
		t.Fatalf("new key during rotation: %v", err)
	}
	if err := Verify([]ed25519.PublicKey{newPub}, ProductEdge, "1.0.0", digest, byOld); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("old key after rotation: %v", err)
	}
}

func TestNoTrustedKeys(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, 32))
	digest := strings.Repeat("b", 64)
	sig, _ := Sign(priv, ProductEdge, "1.0.0", digest)
	for _, trusted := range [][]ed25519.PublicKey{nil, {}, {ed25519.PublicKey{1, 2, 3}}} {
		if err := Verify(trusted, ProductEdge, "1.0.0", digest, sig); !errors.Is(err, ErrNoTrustedKeys) {
			t.Fatalf("trusted=%v: got %v, want ErrNoTrustedKeys", trusted, err)
		}
	}
}

func TestSidecar(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, 32))
	sig, _ := Sign(priv, ProductEdge, "1.0.0", strings.Repeat("c", 64))
	if got, err := SidecarSignature(SidecarContent(sig)); err != nil || got != sig {
		t.Fatalf("round trip: %q %v", got, err)
	}
	for _, bad := range []string{sig, sig + "\n\n", " " + sig + "\n", sig + "\r\n", "\n"} {
		if _, err := SidecarSignature(bad); !errors.Is(err, ErrBadEncoding) {
			t.Fatalf("sidecar %q accepted (err %v)", bad, err)
		}
	}
}

func TestFileSHA256(t *testing.T) {
	got, err := FileSHA256(strings.NewReader("ramio"))
	sum := sha256.Sum256([]byte("ramio"))
	if err != nil || got != hex.EncodeToString(sum[:]) {
		t.Fatalf("got %s %v", got, err)
	}
}

func TestSignRejectsMalformedInputs(t *testing.T) {
	priv := ed25519.NewKeyFromSeed(make([]byte, 32))
	if _, err := Sign(priv[:10], ProductEdge, "1.0.0", strings.Repeat("d", 64)); err == nil {
		t.Fatal("short private key accepted")
	}
	if _, err := Sign(priv, ProductEdge, "1.0.0", "xyz"); !errors.Is(err, ErrBadDigest) {
		t.Fatalf("bad digest: %v", err)
	}
}

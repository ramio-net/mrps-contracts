package capability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// SchemaVersion3 is the capability document with two sets in one signed object.
//
// It exists because the free set has to survive the paid one expiring. Schema 2 put
// one set of limits at the top level with a single valid_until, so when the paid
// right ran out there was nothing left to read: a venue whose trial ended offline
// fell to an emergency floor of two cameras and thirty minutes instead of the free
// composition it was promised. Here Registered carries that composition, and it has
// no expiry of its own by design.
const SchemaVersion3 = 3

// signingDomainV3 separates this document's signatures from every other signature in
// the system, so bytes that verify here cannot be replayed as something else.
const signingDomainV3 = "mrps-capability-schema-3"

// ProfileV3 is the signed capability document.
//
// Signature is the only field excluded from what is signed. Everything else is
// covered, including SchemaVersion and KeyID: a document must not be re-readable as
// another schema, nor attributable to another key, without breaking.
type ProfileV3 struct {
	SchemaVersion int       `json:"schema_version"`
	ProfileID     string    `json:"profile_id"`
	Revision      int64     `json:"revision"`
	Subject       SubjectV3 `json:"subject"`
	IssuedAt      time.Time `json:"issued_at"`
	// OfflineWindowSec is how long the PAID set may be applied without reaching
	// Cloud. It bounds the paid composition, never the ability to work: past it a
	// venue steps down to Registered, which does not expire. Policy data, not a
	// constant — a subscription can buy a longer window.
	OfflineWindowSec int `json:"offline_window_sec"`
	// Registered is mandatory and never expires. A document without a usable
	// Registered block is not acceptable at all, because the step down would have
	// nowhere to land.
	Registered CapabilitySetV3 `json:"registered"`
	// Full is the paid set, absent when nothing is paid for.
	Full      *PaidCapabilitySetV3 `json:"full,omitempty"`
	KeyID     string               `json:"key_id"`
	Signature string               `json:"signature,omitempty"`
}

// SubjectV3 names the installation a document was issued to. All three are checked:
// a valid signature only proves Cloud issued the document, and the subject proves it
// was issued to THIS venue. Without the second check, copying the file from a paying
// installation grants its limits, and the file verifies perfectly.
type SubjectV3 struct {
	InstallationID string `json:"installation_id"`
	EdgeID         string `json:"edge_id"`
	OrgID          string `json:"org_id"`
}

type CapabilitySetV3 struct {
	Limits   Limits   `json:"limits"`
	Features Features `json:"features"`
	// ConditionsRevision names the edition of terms this set was issued under, so a
	// later edit of the platform template cannot silently change what someone was
	// already promised.
	ConditionsRevision int64 `json:"conditions_revision"`
}

type PaidCapabilitySetV3 struct {
	Limits             Limits    `json:"limits"`
	Features           Features  `json:"features"`
	ConditionsRevision int64     `json:"conditions_revision"`
	ValidFrom          time.Time `json:"valid_from"`
	ValidUntil         time.Time `json:"valid_until"`
}

// CanonicalBytesV3 canonicalises a document AS RECEIVED, minus the signature.
//
// It takes raw bytes rather than a struct on purpose, and this is the difference
// that matters most in the file. Rebuilding the document from a Go type drops every
// field this build does not know about — so a document carrying an optional
// extension added later would be canonicalised into something shorter than what was
// signed, and would fail verification on exactly the venues that had not been
// updated. Working from the received bytes keeps unknown signable fields in the
// signature where they belong.
//
// Duplicate keys are rejected before anything else. Given {"a":1,"a":2} decoders
// disagree about which wins, and a document that means different things to the
// signer and the verifier is not a document.
func CanonicalBytesV3(raw []byte) ([]byte, error) {
	return canonicalWithout(raw, "signature")
}

// canonicalWithout is shared by the capability document and the key manifest: both
// are signed objects that exclude exactly one field, and both must refuse the same
// malformed input.
func canonicalWithout(raw []byte, omit string) ([]byte, error) {
	if err := rejectDuplicateKeys(raw); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("parse capability document: %w", err)
	}
	// More() is not enough here, and Cloud's independent verifier caught it: for
	// {"a":1}} the stray brace reads as a closing delimiter rather than as another
	// value, so More() says there is nothing left and the garbage slips through.
	// Demanding EOF is the only form that refuses everything after the document.
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing content after capability document")
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("signed document must be a JSON object")
	}
	delete(obj, omit)

	var out bytes.Buffer
	if err := writeCanonical(&out, obj); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// SigningInputV3 is what Ed25519 actually signs.
func SigningInputV3(canonical []byte) []byte {
	return signingInput(signingDomainV3, canonical)
}

// rejectDuplicateKeys walks the token stream, because encoding/json silently keeps
// the last value for a repeated key and would hide the disagreement.
func rejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return walkForDuplicates(dec)
}

func walkForDuplicates(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyTok.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate key %q in capability document", key)
			}
			seen[key] = true
			if err := walkForDuplicates(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil { // closing }
			return err
		}
	case '[':
		for dec.More() {
			if err := walkForDuplicates(dec); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil { // closing ]
			return err
		}
	}
	return nil
}

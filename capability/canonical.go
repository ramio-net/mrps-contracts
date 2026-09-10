package capability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The canonical form is what two independent implementations must produce
// byte-for-byte from the same document, forever. Every rule below exists because
// the obvious implementation in one language disagrees with the obvious
// implementation in another, and that disagreement surfaces as a venue which
// cannot verify its own capability document.
//
// Measured, not hypothetical. Go's encoding/json escapes the three characters
// < > & into backslash-u escapes, and escapes U+2028 the same way; JavaScript's
// JSON.stringify emits all four literally. Go formats 1000000.0 as 1e+06 and 1.5e-7 as 1.5e-07;
// JavaScript gives 1000000 and 1.5e-7. Any one of those in any string or number
// of a signed document is a signature that verifies on one side and fails on the
// other.
//
// The rules:
//
//   - Strings carry the minimum escaping JSON requires — quote, backslash, and the
//     control characters below 0x20 — and everything else as literal UTF-8. That is
//     what JSON.stringify does, so an implementation in any language agrees with
//     this one without special cases.
//   - Numbers are whole numbers only, within ±(2^53 − 1). Floating point is refused
//     rather than formatted: agreeing on integers is trivial, agreeing on float
//     spelling is a standing invitation to diverge, and nothing in a capability
//     document is fractional — every value is a count and every instant is an RFC
//     3339 string. The range is JavaScript's exact-integer range because a larger
//     value does not survive JSON.parse: 9223372036854775807 comes back as
//     9223372036854776000, canonicalises differently, and the venue is told its
//     signature is invalid while the actual fault is a number.
//   - Object keys are ASCII. Go sorts strings by byte, RFC 8785 by UTF-16 code
//     unit; they agree on ASCII and can disagree above the BMP. The rule removes
//     the question instead of answering it.
//   - Escaped surrogate halves are refused before decoding, because Go silently
//     replaces a lone surrogate with U+FFFD while JavaScript keeps it — a silent
//     change of content inside a signature check.

// CanonicalBytes returns RFC 8785-style canonical JSON for a profile with the
// signature cleared. It is deliberately independent from Go struct field order.
func CanonicalBytes(p Profile) ([]byte, error) {
	cp := p
	cp.Signature = ""

	raw, err := json.Marshal(cp)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, v); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeCanonical(out *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if x {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		if err := writeCanonicalString(out, x); err != nil {
			return err
		}
	case json.Number:
		s, err := canonicalNumber(x.String())
		if err != nil {
			return err
		}
		out.WriteString(s)
	case []any:
		out.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := checkASCIIKey(k); err != nil {
				return err
			}
			if err := writeCanonicalString(out, k); err != nil {
				return err
			}
			out.WriteByte(':')
			if err := writeCanonical(out, x[k]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON value %T", v)
	}
	return nil
}

func canonicalNumber(s string) (string, error) {
	if strings.ContainsAny(s, ".eE") {
		return "", fmt.Errorf("a signed document carries whole numbers only; %q is not one", s)
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil || i > maxSafeInteger || i < -maxSafeInteger {
		return "", fmt.Errorf("number %q is outside the range a signed document may "+
			"carry (±%d)", s, maxSafeInteger)
	}
	return strconv.FormatInt(i, 10), nil
}

// maxSafeInteger is 2^53 − 1: the largest whole number every JSON implementation
// holds exactly. Beyond it JavaScript rounds silently, which inside a signature check
// means a wrong answer reported as a bad signature.
const maxSafeInteger = 1<<53 - 1

// writeCanonicalString emits the minimum escaping JSON requires and nothing more,
// which is what every other language's default already produces.
func writeCanonicalString(out *bytes.Buffer, s string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("string in a signed document is not valid UTF-8")
	}
	out.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(out, `\u%04x`, r)
				continue
			}
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return nil
}

func checkASCIIKey(k string) error {
	for i := 0; i < len(k); i++ {
		if k[i] < 0x20 || k[i] > 0x7e {
			return fmt.Errorf("object key %q is not ASCII; a signed document must not "+
				"depend on how an implementation orders non-ASCII keys", k)
		}
	}
	return nil
}

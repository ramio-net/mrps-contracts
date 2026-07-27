package capability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

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
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		out.Write(b)
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
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			out.Write(kb)
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
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return "", err
		}
		return strconv.FormatFloat(f, 'g', -1, 64), nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return strconv.FormatInt(i, 10), nil
	}
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return "", err
	}
	return strconv.FormatUint(u, 10), nil
}

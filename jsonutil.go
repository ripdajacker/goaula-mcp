package aula

import (
	"bytes"
	"encoding/json"
	"strings"
)

// MarshalJSON marshals v to compact JSON without HTML escaping (Jackson-compatible),
// so HTML content from Aula is not mangled into <p> etc.
func MarshalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// unmarshalJSON is a small wrapper around json.Unmarshal.
func unmarshalJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// jsonString extracts data as a plain string (used for aulaToken responses where the
// data element is a JSON string).
func jsonString(data []byte) (string, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

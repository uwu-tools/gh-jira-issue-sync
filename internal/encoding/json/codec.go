package json

import (
	"encoding/json"
	"fmt"
)

// Codec implements the encoding.Encoder interface for JSON encoding.
type Codec struct {
	// prefix for JSON marshal.
	Prefix string

	// indentation for JSON marshal.
	Indent string
}

func (c *Codec) Encode(v map[string]interface{}) ([]byte, error) {
	b, err := json.MarshalIndent(v, c.Prefix, c.Indent)
	if err != nil {
		return nil, fmt.Errorf("json marshalling value: %w", err)
	}

	return b, nil
}

package toml

import (
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

// Codec implements the encoding.Encoder interface for TOML encoding.
type Codec struct{}

func (Codec) Encode(v map[string]interface{}) ([]byte, error) {
	b, err := toml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("toml marshalling value: %w", err)
	}

	return b, nil
}

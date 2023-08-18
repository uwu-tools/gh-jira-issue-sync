package yaml

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Codec implements the encoding.Encoder interfaces for YAML encoding.
type Codec struct{}

func (Codec) Encode(v map[string]interface{}) ([]byte, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("yaml marshalling value: %w", err)
	}

	return b, nil
}

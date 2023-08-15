package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfigTypeFromName(t *testing.T) {
	tests := []*struct {
		name, filename, expected string
	}{
		{
			name:     "empty name",
			filename: "",
			expected: "json",
		},
		{
			name:     "json",
			filename: "config.json",
			expected: "json",
		},
		{
			name:     "toml",
			filename: "config.toml",
			expected: "toml",
		},
		{
			name:     "yaml",
			filename: "config.yaml",
			expected: "yaml",
		},
		{
			name:     "any file type",
			filename: "config.xyz",
			expected: "xyz",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Assert config type with %s", test.name), func(t *testing.T) {
			assert.Equal(t, test.expected, getConfigTypeFromName(test.filename))
		})
	}
}

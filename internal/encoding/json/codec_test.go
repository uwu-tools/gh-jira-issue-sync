package json

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const encoded = `{
  "key": "value",
  "list": [
    "item1",
    "item2",
    "item3"
  ],
  "map": {
    "key": "value"
  },
  "nested_map": {
    "map": {
      "key": "value",
      "list": [
        "item1",
        "item2",
        "item3"
      ]
    }
  }
}`

var data = map[string]interface{}{
	"key": "value",
	"list": []interface{}{
		"item1",
		"item2",
		"item3",
	},
	"map": map[string]interface{}{
		"key": "value",
	},
	"nested_map": map[string]interface{}{
		"map": map[string]interface{}{
			"key": "value",
			"list": []interface{}{
				"item1",
				"item2",
				"item3",
			},
		},
	},
}

func TestEncode(t *testing.T) {
	codec := Codec{
		Indent: "  ",
	}

	b, err := codec.Encode(data)
	require.Empty(t, err)
	require.Equal(t, encoded, string(b))
}

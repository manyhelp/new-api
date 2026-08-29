package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntValueUnmarshalJSONAcceptsNumberAndString guards the regression where
// ali ``usage.duration`` (and any other IntValue field) returned a fractional
// JSON number, e.g. ``5.0`` or ``5.5``, and the previous int-only first branch
// failed, cascading into a whole-struct unmarshal error that left video tasks
// stuck "in progress" forever.
func TestIntValueUnmarshalJSONAcceptsNumberAndString(t *testing.T) {
	cases := []struct {
		name string
		body string
		want IntValue
	}{
		{"integer_number", `{"d":5}`, 5},
		{"float_number", `{"d":5.0}`, 5},
		{"fractional_number", `{"d":5.5}`, 5}, // truncated toward zero, matching Atoi semantics
		{"zero", `{"d":0}`, 0},
		{"string_integer", `{"d":"5"}`, 5},
		{"string_zero", `{"d":"0"}`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var v struct {
				D IntValue `json:"d"`
			}
			require.NoError(t, json.Unmarshal([]byte(c.body), &v), "body=%s", c.body)
			assert.Equal(t, c.want, v.D, "body=%s", c.body)
		})
	}

	require.NoError(t, json.Unmarshal([]byte(`{}`), &struct {
		D IntValue `json:"d,omitempty"`
	}{}), "absent field must not error")
}

func TestIntValueMarshalJSONEmitsInteger(t *testing.T) {
	b, err := json.Marshal(IntValue(7))
	require.NoError(t, err)
	assert.Equal(t, "7", string(b))
}

func TestIntValueUnmarshalJSONRejectsGarbage(t *testing.T) {
	var v IntValue
	err := json.Unmarshal([]byte(`"not-a-number"`), &v)
	require.Error(t, err)
}

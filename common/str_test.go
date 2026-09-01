package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTruncateRunes guards the rune-safety contract: truncation must never
// split a multi-byte CJK character and must append "..." only when it
// actually truncates. Image/video log `content` fields rely on this to stay
// readable and safe for varchar columns regardless of script — a regression
// to byte-slicing would both cut a CJK char in half (mojibake) and produce a
// wrong prefix length.
func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  string
	}{
		{"empty", "", 10, ""},
		{"under limit no ellipsis", "abc", 10, "abc"},
		{"exact limit no ellipsis", "abc", 3, "abc"},
		{"ascii truncate", "abcdef", 3, "abc..."},
		{"cjk truncate no split", "你好世界你好世界", 4, "你好世界..."},
		{"cjk under limit", "你好", 5, "你好"},
		{"mixed ascii and cjk", "a你好b", 3, "a你好..."},
		{"zero max returns original", "你好abc", 0, "你好abc"},
		{"negative max returns original", "abc", -1, "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TruncateRunes(tt.input, tt.max))
		})
	}
}

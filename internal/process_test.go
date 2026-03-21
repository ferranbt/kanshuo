package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPinyinToneConversion(t *testing.T) {
	// test cases that panic in cedic pinyin tone conversion
	cases := []struct {
		input, output string
	}{
		{"lu:e4", "luè"},
		{"4zu3 chen2 she4", "zǔ chén shè"},
	}
	for _, c := range cases {
		pinyin := convertToPinyinTones(c.input)
		require.Equal(t, pinyin, c.output)
	}
}

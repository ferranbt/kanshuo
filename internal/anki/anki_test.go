package anki

import (
	"testing"

	"github.com/ferranbt/kanshuo/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestAnki_SaveWordSimple(t *testing.T) {
	client := NewClient(testutil.NewTestLogger())

	word := Word{
		Text:    "学习",
		Pinyin:  "xuéxí",
		Meaning: "to study, to learn",
		Pos:     "verb",
	}

	err := client.SaveWord("kanshuo", word)
	require.NoError(t, err)
}

func TestAnki_SaveWordWithAudio(t *testing.T) {
	client := NewClient(testutil.NewTestLogger())

	word := Word{
		Text:    "你好世界",
		Pinyin:  "nǐ hǎo shìjiè",
		Meaning: "hello world",
		Pos:     "phrase",
	}

	err := client.SaveWord("kanshuo", word)
	require.NoError(t, err)
}

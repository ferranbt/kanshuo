package ocr

import (
	"fmt"
	"testing"

	"github.com/ferranbt/kanshuo/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestEasyOCR(t *testing.T) {
	logger := testutil.NewTestLogger()

	ocr, err := NewEasyOCR(logger, true)
	require.NoError(t, err)

	defer ocr.Stop()

	cases := []struct {
		frameName string
	}{
		{"./testdata/frame_01.jpg"},
		{"./testdata/frame_02.jpg"},
		// {"./testdata/frame_03.jpg", false},
		{"./testdata/frame_04.jpg"},
		{"./testdata/frame_05.jpg"},
		{"./testdata/frame_06.jpg"},
		{"./testdata/frame_07.jpg"},
		{"./testdata/frame_08.jpg"},
	}

	for _, m := range cases {
		fmt.Println("---")
		fmt.Println(ocr.OCR(m.frameName))

		cropped, err := testutil.CropBottomQuarter(m.frameName)
		require.NoError(t, err)

		fmt.Println(cropped)
		fmt.Println(ocr.OCR(cropped))
	}
}

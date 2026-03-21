package ocr

import (
	"fmt"
	"testing"

	"github.com/ferranbt/kanshuo/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestEasyOCR(t *testing.T) {
	logger := testutil.NewTestLogger()

	ocr, err := NewEasyOCR(logger, false)
	require.NoError(t, err)

	defer ocr.Stop()

	fmt.Println(ocr.OCR("./testdata/frame_01.jpg"))
	fmt.Println(ocr.OCR("./testdata/frame_02.jpg"))
	fmt.Println(ocr.OCR("./testdata/frame_03.png"))
	fmt.Println(ocr.OCR("./testdata/frame_04.jpg"))
}

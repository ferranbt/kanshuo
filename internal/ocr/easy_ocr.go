package ocr

import (
	_ "embed"
	"log/slog"
	"net/http"
	"time"

	"github.com/ferranbt/kanshuo/internal/python"
	"github.com/ferranbt/kanshuo/internal/testutil"
)

var (
	ocrServiceURL = "http://localhost:5010"
)

type OCRResult struct {
	Regions         []OCRRegion `json:"regions"`
	TimeToProcessMs int64       `json:"time"`
}

type OCRRegion struct {
	Text       string  `json:"text"`
	X1         int     `json:"x1"`
	Y1         int     `json:"y1"`
	X2         int     `json:"x2"`
	Y2         int     `json:"y2"`
	Confidence float64 `json:"confidence"`
}

//go:embed easy_ocr_script.py
var ocrServiceScript string

type EasyOCR struct {
	logger *slog.Logger
	srv    *python.PythonService
	clt    *testutil.HttpClient
}

func NewEasyOCR(logger *slog.Logger, traditional bool) (*EasyOCR, error) {
	args := []string{}
	if traditional {
		args = append(args, "--traditional")
	}

	ocrSvc, err := python.New(logger, "ocr", ocrServiceScript, args...)
	if err != nil {
		return nil, err
	}

	clt := testutil.NewHTTPClient(ocrServiceURL)
	if err := clt.WaitToReady(10 * time.Second); err != nil {
		return nil, err
	}

	return &EasyOCR{logger: logger, srv: ocrSvc, clt: clt}, nil
}

func (e *EasyOCR) Stop() error {
	return e.srv.Close()
}

func (e *EasyOCR) OCR(imagePath string) (*OCRResult, error) {
	start := time.Now()

	reqBody := map[string]string{
		"image_path": imagePath,
	}

	var result struct {
		OK      bool        `json:"ok"`
		Regions []OCRRegion `json:"regions"`
		Error   string      `json:"error"`
	}

	if err := e.clt.DoRequest(&result, "/ocr", http.MethodPost, reqBody); err != nil {
		return nil, err
	}

	now := time.Since(start)
	e.logger.Debug("time to process ocr", "elapsed", time.Since(start))

	res := &OCRResult{
		Regions:         result.Regions,
		TimeToProcessMs: now.Milliseconds(),
	}
	return res, nil
}

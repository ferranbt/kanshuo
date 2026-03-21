package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HttpClient struct {
	endpoint string
}

func NewHTTPClient(endpoint string) *HttpClient {
	return &HttpClient{endpoint: endpoint}
}

func (h *HttpClient) WaitToReady(timeout time.Duration) error {
	timeoutT := time.NewTimer(timeout)

	for {
		select {
		case <-timeoutT.C:
			return fmt.Errorf("timeout")

		case <-time.After(100 * time.Millisecond):
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, h.endpoint+"/health", nil)
			if err != nil {
				continue
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
	}
}

func (h *HttpClient) DoRequest(respData interface{}, path string, method string, input interface{}) error {
	url := h.endpoint + path

	var (
		resp *http.Response
		err  error
	)

	var inputData []byte
	if inputMap, ok := input.(map[string]string); ok {
		if inputData, err = json.Marshal(inputMap); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unknown input type")
	}

	switch method {
	case http.MethodPost:
		resp, err = http.Post(url, "application/json", bytes.NewReader(inputData))
	case http.MethodGet:
		resp, err = http.Get(url)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, respData); err != nil {
		return err
	}
	return nil
}

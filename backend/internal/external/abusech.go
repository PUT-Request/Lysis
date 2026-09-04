package external

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"lysis/internal/config"
)

type MBClient struct {
	apiKey     string
	httpClient *http.Client
}

type MBResponse struct {
	QueryStatus string `json:"query_status"`
	Data        []struct {
		SHA256Hash   string `json:"sha256_hash"`
		Signature    string `json:"signature"`
		FileType     string `json:"file_type"`
		FirstSeen    string `json:"first_seen"`
		Reporter     string `json:"reporter"`
		FileName     string `json:"file_name"`
	} `json:"data"`
}

func NewMBClient(cfg config.ExternalAPI) *MBClient {
	return &MBClient{
		apiKey:     cfg.APIKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *MBClient) LookupHash(hash string) (string, error) {
	reqBody := map[string]string{
		"query":  "get_info",
		"hash":   hash,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", "https://mb-api.abuse.ch/api/v1/", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("API-KEY", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var mbResp MBResponse
	if err := json.Unmarshal(body, &mbResp); err != nil {
		return "", err
	}

	if mbResp.QueryStatus != "ok" || len(mbResp.Data) == 0 {
		return "", nil
	}

	resultJSON, _ := json.Marshal(mbResp.Data[0])
	return string(resultJSON), nil
}

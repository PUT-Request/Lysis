package external

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"lysis/internal/config"
)

type VTClient struct {
	apiKey    string
	endpoint  string
	throttle  time.Duration
	lastCall  time.Time
	mu        sync.Mutex
	httpClient *http.Client
}

type VTResponse struct {
	Data struct {
		Attributes struct {
			LastAnalysisStats map[string]int `json:"last_analysis_stats"`
			PopularThreat     struct {
				Name string `json:"suggested_threat_label"`
			} `json:"popular_threat_classification"`
		} `json:"attributes"`
	} `json:"data"`
}

func NewVTClient(cfg config.ExternalAPI) *VTClient {
	return &VTClient{
		apiKey:    cfg.APIKey,
		endpoint:  "https://www.virustotal.com/api/v3",
		throttle:  time.Minute / time.Duration(cfg.ThrottleRPM),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *VTClient) LookupFile(hash string) (string, error) {
	c.mu.Lock()
	if !c.lastCall.IsZero() {
		elapsed := time.Since(c.lastCall)
		if elapsed < c.throttle {
			time.Sleep(c.throttle - elapsed)
		}
	}
	c.lastCall = time.Now()
	c.mu.Unlock()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/files/%s", c.endpoint, hash), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-apikey", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 404 {
		return "", nil
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("VT API returned %d: %s", resp.StatusCode, string(body))
	}

	var vtResp VTResponse
	if err := json.Unmarshal(body, &vtResp); err != nil {
		return "", err
	}

	result := map[string]interface{}{
		"stats":    vtResp.Data.Attributes.LastAnalysisStats,
		"threat":   vtResp.Data.Attributes.PopularThreat.Name,
	}
	resultJSON, _ := json.Marshal(result)
	return string(resultJSON), nil
}

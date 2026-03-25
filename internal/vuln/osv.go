package vuln

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const osvBatchURL = "https://api.osv.dev/v1/querybatch"
const maxBatchSize = 1000

type OSVClient struct {
	http *http.Client
}

func NewOSVClient() *OSVClient {
	return &OSVClient{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

type OSVQuery struct {
	Package OSVPackage `json:"package"`
}

type OSVPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

type OSVBatchRequest struct {
	Queries []OSVQuery `json:"queries"`
}

type OSVBatchResponse struct {
	Results []OSVResult `json:"results"`
}

type OSVResult struct {
	Vulns []OSVVuln `json:"vulns"`
}

type OSVVuln struct {
	ID       string        `json:"id"`
	Summary  string        `json:"summary"`
	Aliases  []string      `json:"aliases"`
	Severity []OSVSeverity `json:"severity"`
}

type OSVSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}

func (c *OSVClient) QueryBatch(ctx context.Context, queries []OSVQuery) ([]OSVResult, error) {
	var allResults []OSVResult

	for i := 0; i < len(queries); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(queries) {
			end = len(queries)
		}

		batch := queries[i:end]
		results, err := c.queryBatchChunk(ctx, batch)
		if err != nil {
			return nil, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

func (c *OSVClient) queryBatchChunk(ctx context.Context, queries []OSVQuery) ([]OSVResult, error) {
	reqBody, err := json.Marshal(OSVBatchRequest{Queries: queries})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, osvBatchURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv.dev returned %d: %s", resp.StatusCode, string(body))
	}

	var batchResp OSVBatchResponse
	if err := json.Unmarshal(body, &batchResp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return batchResp.Results, nil
}

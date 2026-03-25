package pagerduty

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ahlert/telvar/internal/config"
)

const baseURL = "https://api.pagerduty.com"

type Client struct {
	http   *http.Client
	apiKey string
}

func NewClient(cfg *config.PagerDutyConfig) *Client {
	return &Client{
		http:   &http.Client{Timeout: 15 * time.Second},
		apiKey: cfg.APIKey,
	}
}

type OnCall struct {
	UserName string
	UserEmail string
	ScheduleName string
	EscalationPolicy string
}

type oncallsResponse struct {
	Oncalls []struct {
		User struct {
			Summary string `json:"summary"`
			Email   string `json:"email"`
		} `json:"user"`
		Schedule struct {
			Summary string `json:"summary"`
		} `json:"schedule"`
		EscalationPolicy struct {
			Summary string `json:"summary"`
		} `json:"escalation_policy"`
	} `json:"oncalls"`
}

func (c *Client) ListOnCallNow(ctx context.Context) ([]OnCall, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		baseURL+"/oncalls?include[]=users&earliest=true&limit=100", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Token token="+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching oncalls: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pagerduty returned %d: %s", resp.StatusCode, string(body))
	}

	var result oncallsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing oncalls response: %w", err)
	}

	var oncalls []OnCall
	for _, oc := range result.Oncalls {
		oncalls = append(oncalls, OnCall{
			UserName:         oc.User.Summary,
			UserEmail:        oc.User.Email,
			ScheduleName:     oc.Schedule.Summary,
			EscalationPolicy: oc.EscalationPolicy.Summary,
		})
	}

	return oncalls, nil
}

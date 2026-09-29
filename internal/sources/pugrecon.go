// Package sources: pugrecon adapter.
//
// Provider: PugRecon. Docs: https://gist.github.com/c3l3si4n/68ac06ebe85f8c0b821800432c7f89a6
// Endpoint: POST https://pugrecon.com/api/v1/domains body {"domain_name":"<d>"} (indexed data; no pagination).
// Auth: "Authorization: Bearer <api_key>". Free quota is tiny (conflicting docs).
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var pugreconBaseURL = "https://pugrecon.com"

type pugrecon struct{}

func init() { Register(pugrecon{}) }

func (pugrecon) Info() Info {
	return Info{
		Name: "pugrecon", URL: "https://pugrecon.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.2, Burst: 1,
	}
}

func (pugrecon) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{"domain_name": domain})
	var resp struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
		Limited        bool   `json:"limited"`
		QuotaRemaining *int64 `json:"quota_remaining"`
		Message        string `json:"message"`
		Error          string `json:"error"`
	}
	err = s.HTTP.PostJSON(ctx, pugreconBaseURL+"/api/v1/domains",
		map[string]string{"Authorization": "Bearer " + key, "Accept": "application/json"}, body, &resp)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return merklemapClassify(resp.Error)
	}
	if len(resp.Results) == 0 {
		l := strings.ToLower(resp.Message)
		if strings.Contains(l, "quota") || strings.Contains(l, "limit") || strings.Contains(l, "credit") {
			return fmt.Errorf("%w: %s", ErrRateLimited, resp.Message)
		}
		if resp.Message != "" && (strings.Contains(l, "invalid") || strings.Contains(l, "error")) {
			return merklemapClassify(resp.Message)
		}
		return nil
	}
	if resp.Limited {
		s.Logf("pugrecon: result list limited by plan (quota_remaining=%v)", resp.QuotaRemaining)
	}
	for _, r := range resp.Results {
		emit(r.Name)
	}
	return nil
}

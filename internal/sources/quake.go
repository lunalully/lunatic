// Package sources: quake adapter (360 Quake).
//
// Provider: 360 Quake. Docs: https://quake.360.net (help page is a JS SPA; token header confirmed
// via https://github.com/360-Quake/quake-mcp).
// Endpoint: POST https://quake.360.net/api/v3/search/quake_service, JSON
// {"query":"domain: \"<d>\"","include":["service.http.host"],"latest":true,"size":N,"start":N}
// (search over indexed data; "latest" only selects newest stored data). Endpoint from Subfinder reference.
// Auth: "X-QuakeToken: <api_key>". Each result consumes account quota.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

var quakeBaseURL = "https://quake.360.net"

var quakeSize = 100

type quake struct{}

func init() { Register(quake{}) }

func (quake) Info() Info {
	return Info{
		Name: "quake", URL: "https://quake.360.net",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.3, Burst: 1,
	}
}

func (quake) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"X-QuakeToken": key, "Accept": "application/json"}
	start := 0
	for page := 0; page < merklemapPages(s); page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{
			"query": `domain: "` + domain + `"`, "include": []string{"service.http.host"},
			"latest": true, "size": quakeSize, "start": start,
		})
		var resp struct {
			Code    any             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
			Meta    struct {
				Pagination struct {
					Total int `json:"total"`
				} `json:"pagination"`
			} `json:"meta"`
		}
		if err := s.HTTP.PostJSON(ctx, quakeBaseURL+"/api/v3/search/quake_service", hdr, body, &resp); err != nil {
			return err
		}
		if c := fmt.Sprint(resp.Code); resp.Code != nil && c != "0" && c != "<nil>" {
			return quakeErr(c, resp.Message)
		}
		var data []struct {
			Service struct {
				HTTP struct {
					Host string `json:"host"`
				} `json:"http"`
			} `json:"service"`
		}
		if len(resp.Data) == 0 || string(resp.Data) == "null" {
			if resp.Code == nil {
				return fmt.Errorf("%w: quake response has neither code nor data", ErrUnexpected)
			}
			return nil
		}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			return fmt.Errorf("%w: quake data field: %v", ErrUnexpected, err)
		}
		if len(data) == 0 {
			return nil
		}
		for _, d := range data {
			emit(d.Service.HTTP.Host)
		}
		start += quakeSize
		if len(data) < quakeSize || start >= resp.Meta.Pagination.Total {
			return nil
		}
	}
	return nil
}

func quakeErr(code, msg string) error {
	if err := merklemapClassify(msg); errors.Is(err, ErrAuth) || errors.Is(err, ErrRateLimited) {
		return err
	}
	return fmt.Errorf("%w: quake code %s: %s", ErrUnexpected, strconv.Quote(code), msg)
}

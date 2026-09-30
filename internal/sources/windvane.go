// Package sources: windvane adapter.
//
// Provider:     Windvane by Lichoin (https://windvane.lichoin.com)
// Docs:         https://windvane.lichoin.com/docs-api (page not readable when
//
//	checked; request/response shape follows subfinder's client and
//	is parsed tolerantly).
//
// Endpoint:     POST {base}/trpc.backendhub.public.WindvaneService/ListSubDomain
//
//	body {"domain":D,"page_request":{"page":N,"count":1000}}
//
// Auth:         API key, header "X-Api-Key".
// Pagination:   page/count; stops on an empty page, a short page, or when
//
//	page*count >= data.page_response.total_count; bounded by MaxPages.
//
// Errors:       200 bodies carry code (0 = ok) and msg.
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

var windvaneBaseURL = "https://windvane.lichoin.com"

var windvaneCount = 1000

type windvane struct{}

func init() { Register(windvane{}) }

func (windvane) Info() Info {
	return Info{
		Name: "windvane", URL: "https://windvane.lichoin.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (windvane) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"X-Api-Key": key}
	for page := 1; page <= s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{
			"domain":       domain,
			"page_request": map[string]int{"page": page, "count": windvaneCount},
		})
		var r struct {
			Code json.Number `json:"code"`
			Msg  string      `json:"msg"`
			Data struct {
				List         []json.RawMessage `json:"list"`
				PageResponse struct {
					Total json.Number `json:"total_count"`
				} `json:"page_response"`
			} `json:"data"`
		}
		if err := s.HTTP.PostJSON(ctx, windvaneBaseURL+"/trpc.backendhub.public.WindvaneService/ListSubDomain", hdr, body, &r); err != nil {
			return err
		}
		if r.Code == "" {
			return fmt.Errorf("%w: windvane: code missing", ErrUnexpected)
		}
		if r.Code.String() != "0" {
			l := strings.ToLower(r.Msg)
			switch {
			case strings.Contains(l, "key") || strings.Contains(l, "auth") || strings.Contains(l, "token") || strings.Contains(l, "permission"):
				return fmt.Errorf("%w: windvane: %s", ErrAuth, r.Msg)
			case strings.Contains(l, "limit") || strings.Contains(l, "quota") || strings.Contains(l, "frequen"):
				return fmt.Errorf("%w: windvane: %s", ErrRateLimited, r.Msg)
			}
			return fmt.Errorf("%w: windvane: code %s: %s", ErrUnexpected, r.Code, r.Msg)
		}
		for _, d := range r.Data.List {
			var o struct {
				Domain string `json:"domain"`
			}
			var str string
			if json.Unmarshal(d, &o) == nil && o.Domain != "" {
				emit(o.Domain)
			} else if json.Unmarshal(d, &str) == nil && str != "" {
				emit(str)
			}
		}
		total, _ := r.Data.PageResponse.Total.Int64()
		if len(r.Data.List) < windvaneCount || (total > 0 && int64(page)*int64(windvaneCount) >= total) {
			return nil
		}
	}
	return nil
}

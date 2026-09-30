// Package sources: threatminer adapter.
//
// Provider:     ThreatMiner (https://www.threatminer.org)
// Docs:         https://www.threatminer.org/api.php
// Endpoint:     GET {base}/v2/domain.php?q={domain}&rt=5   (rt=5 = subdomains)
// Response:     JSON {"status_code":"200","status_message":"...","results":["a.example.com",...]}.
//
//	status_code "404" (no results) is success with zero names; any
//	other non-200 code in the body is an error. The HTTP status may be 200
//	in both cases, so the body code is authoritative.
//
// Auth:         none. Documented limit: 10 queries per minute (RPS 0.1).
// Pagination:   none; one request per domain.
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check
// (the docs page did not render the rt=5 example; format follows the well-known API shape).
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var threatminerBaseURL = "https://api.threatminer.org"

type threatminer struct{}

func init() { Register(threatminer{}) }

func (threatminer) Info() Info {
	return Info{Name: "threatminer", URL: "https://www.threatminer.org", Auth: AuthNone, Default: true, RPS: 0.1, Burst: 1}
}

func (threatminer) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	var r struct {
		Code    json.RawMessage `json:"status_code"`
		Message string          `json:"status_message"`
		Results []string        `json:"results"`
	}
	u := threatminerBaseURL + "/v2/domain.php?q=" + url.QueryEscape(domain) + "&rt=5"
	if err := s.HTTP.GetJSON(ctx, u, nil, &r); err != nil {
		if g2status(err) == 404 {
			return nil
		}
		return err
	}
	code := strings.Trim(strings.TrimSpace(string(r.Code)), `"`)
	switch code {
	case "200":
	case "404":
		return nil
	case "429":
		return fmt.Errorf("%w: threatminer: %.80s", ErrRateLimited, r.Message)
	case "":
		return fmt.Errorf("%w: threatminer: missing status_code", ErrUnexpected)
	default:
		return fmt.Errorf("%w: threatminer status_code %s: %.80s", ErrUnexpected, code, r.Message)
	}
	for _, h := range r.Results {
		emit(h)
	}
	return nil
}

// Provider: Driftnet (https://driftnet.io).
// Docs: https://driftnet.io/api-docs (parameters field, summarize, summary_context, summary_limit; 200 = data, 204 = no results).
// Endpoints (searches of stored CT/scan/DNS datasets only; no scan submission): GET https://api.driftnet.io/v1/<ep>
// ?field=host:<domain>&summarize=host&summary_context=<ctx>&summary_limit=10000 for ep in driftnetEndpoints.
// The per-endpoint summary_context values come from third-party code and are NOT confirmed by the docs; an endpoint
// that answers 4xx is skipped, and the source fails only if every endpoint failed.
// Auth: required; header Authorization: Bearer <token>. Response: {"summary":{"other":N,"values":{"<host>":count}}}.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

var driftnetBaseURL = "https://api.driftnet.io"

var driftnetEndpoints = []struct{ path, ctx string }{
	{"ct/log", "cert-dns-name"},
	{"scan/protocols", "cert-dns-name"},
	{"scan/domains", "host"},
	{"domain/rdns", "host"},
}

type driftnet struct{}

func init() { Register(driftnet{}) }

func (driftnet) Info() Info {
	return Info{Name: "driftnet", URL: "https://driftnet.io", Auth: AuthRequired,
		CredFields: []string{"api_key"}, Default: true, RPS: 1, Burst: 1}
}

func (driftnet) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	headers := map[string]string{"Authorization": "Bearer " + key, "Accept": "application/json"}
	ok, failed := 0, 0
	var lastErr error
	for i, ep := range driftnetEndpoints {
		if i >= s.MaxPages {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		q := url.Values{"field": {"host:" + domain}, "summarize": {"host"}, "summary_context": {ep.ctx}, "summary_limit": {strconv.Itoa(10000)}}
		resp, err := s.HTTP.Get(ctx, driftnetBaseURL+"/v1/"+ep.path+"?"+q.Encode(), headers)
		if err != nil {
			if errors.Is(err, ErrAuth) || errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTimeout) || errors.Is(err, ErrBlockedHost) {
				return err
			}
			failed++
			lastErr = err
			s.Logf("driftnet %s: %v", ep.path, err)
			continue
		}
		ok++
		if resp.StatusCode == 204 || len(resp.Body) == 0 {
			continue
		}
		var out struct {
			Summary struct {
				Values map[string]json.RawMessage `json:"values"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(resp.Body, &out); err != nil {
			failed++
			ok--
			lastErr = fmt.Errorf("%w: driftnet invalid JSON: %v", ErrUnexpected, err)
			continue
		}
		for h := range out.Summary.Values {
			emit(h)
		}
	}
	if ok == 0 && failed > 0 {
		return lastErr
	}
	return nil
}

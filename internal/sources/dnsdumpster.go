// Provider: DNSdumpster (https://dnsdumpster.com), owned by HackerTarget.
// Docs: https://dnsdumpster.com/developer/
// Endpoint: GET https://api.dnsdumpster.com/domain/<domain>
// Auth: required; header X-API-Key (free accounts can create a key: 50 requests/day, 50 records/domain;
// rate limit 1 request per 2 seconds).
// Response: JSON with record arrays a, cname, mx, ns (each item has "host"); only host names are emitted, IPs and
// banner/geo data are dropped. Only the first page is requested: the paging response shape is not documented.
// Passive nature: the docs do not state explicitly whether lookups are served from stored data (unconfirmed).
// Date checked: 2026-09-29.
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

var dnsdumpsterBaseURL = "https://api.dnsdumpster.com"

type dnsdumpster struct{}

func init() { Register(dnsdumpster{}) }

func (dnsdumpster) Info() Info {
	return Info{Name: "dnsdumpster", URL: "https://dnsdumpster.com", Auth: AuthRequired,
		CredFields: []string{"api_key"}, Default: true, RPS: 0.5, Burst: 1}
}

type dnsdumpsterRec struct {
	Host string `json:"host"`
}

func (dnsdumpster) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var out struct {
		Error  string           `json:"error"`
		A      []dnsdumpsterRec `json:"a"`
		CNAME  []dnsdumpsterRec `json:"cname"`
		MX     []dnsdumpsterRec `json:"mx"`
		NS     []dnsdumpsterRec `json:"ns"`
		Errmsg string           `json:"message"`
	}
	if err := s.HTTP.GetJSON(ctx, dnsdumpsterBaseURL+"/domain/"+url.PathEscape(domain), map[string]string{"X-API-Key": key}, &out); err != nil {
		return err
	}
	if out.Error != "" {
		low := strings.ToLower(out.Error)
		switch {
		case strings.Contains(low, "rate limit"), strings.Contains(low, "quota"):
			return fmt.Errorf("%w: dnsdumpster: %.80s", ErrRateLimited, out.Error)
		case strings.Contains(low, "api key"), strings.Contains(low, "unauthor"), strings.Contains(low, "forbidden"):
			return fmt.Errorf("%w: dnsdumpster: %.80s", ErrAuth, out.Error)
		}
		return fmt.Errorf("%w: dnsdumpster error: %.80s", ErrUnexpected, out.Error)
	}
	for _, set := range [][]dnsdumpsterRec{out.A, out.CNAME, out.MX, out.NS} {
		for _, r := range set {
			if r.Host != "" {
				emit(strings.TrimSuffix(r.Host, "."))
			}
		}
	}
	return nil
}

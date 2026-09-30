// Package sources: shodan adapter.
//
// Provider:     Shodan (https://www.shodan.io)
// Docs:         https://developer.shodan.io/api  (GET /dns/domain/{domain})
// Endpoint:     GET {base}/dns/domain/{domain}?key=KEY&page=N
// Auth:         API key, query parameter "key" (the only method the docs show).
//
//	Used by default only when a key is configured.
//
// Pagination:   "page" (1-based); the response carries "more": true while
//
//	further pages exist. Bounded by Session.MaxPages.
//
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/lunalully/lunatic/internal/httpx"
)

var shodanBaseURL = "https://api.shodan.io"

type shodan struct{}

func init() { Register(shodan{}) }

func (shodan) Info() Info {
	return Info{
		Name: "shodan", URL: "https://www.shodan.io",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

type shodanResp struct {
	Domain     string   `json:"domain"`
	Subdomains []string `json:"subdomains"`
	Data       []struct {
		Subdomain string `json:"subdomain"`
	} `json:"data"`
	More  bool   `json:"more"`
	Error string `json:"error"`
}

// shodanErr maps a provider error message found in a body to a typed error.
func shodanErr(msg string) error {
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "api key") || strings.Contains(l, "unauthorized") || strings.Contains(l, "access denied"):
		return fmt.Errorf("%w: shodan: %s", ErrAuth, msg)
	case strings.Contains(l, "rate limit") || strings.Contains(l, "too many"):
		return fmt.Errorf("%w: shodan: %s", ErrRateLimited, msg)
	}
	return fmt.Errorf("%w: shodan: %s", ErrUnexpected, msg)
}

func (shodan) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	for page := 1; page <= s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := shodanBaseURL + "/dns/domain/" + url.PathEscape(domain) + "?key=" + url.QueryEscape(key) + "&page=" + strconv.Itoa(page)
		var r shodanResp
		if err := s.HTTP.GetJSON(ctx, u, nil, &r); err != nil {
			var he *httpx.Error
			if errors.As(err, &he) && he.StatusCode == 404 { // no information for this domain
				return nil
			}
			return err
		}
		if r.Error != "" {
			return shodanErr(r.Error)
		}
		emitLabel := func(l string) {
			l = strings.TrimSpace(l)
			if l == "" || l == "@" {
				return
			}
			if strings.HasSuffix(l, "."+domain) {
				emit(l)
			} else {
				emit(l + "." + domain)
			}
		}
		for _, l := range r.Subdomains {
			emitLabel(l)
		}
		for _, d := range r.Data {
			emitLabel(d.Subdomain)
		}
		if !r.More {
			break
		}
	}
	return nil
}

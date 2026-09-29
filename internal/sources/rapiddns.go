// Package sources: rapiddns adapter.
//
// Provider: RapidDNS. Site: https://rapiddns.io (robots.txt allows crawling; ToS not read).
// Endpoint: GET https://rapiddns.io/subdomain/<d>?page=<n>&full=1 (public HTML page; names extracted
// with scope.FindInText). Endpoint from Subfinder reference; the official Pro JSON API
// (/api/search/..., X-API-KEY) is documented only superficially and is not implemented.
// Auth: none. Not in the default run (unofficial scraping). Any CAPTCHA/challenge or block is
// terminal (ErrUnavailable); it is never bypassed.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/lunalully/lunatic/internal/httpx"
	"github.com/lunalully/lunatic/internal/scope"
)

var rapiddnsBaseURL = "https://rapiddns.io"

type rapiddns struct{}

func init() { Register(rapiddns{}) }

func (rapiddns) Info() Info {
	return Info{Name: "rapiddns", URL: "https://rapiddns.io", Auth: AuthNone, Default: false, RPS: 0.2, Burst: 1}
}

func rapiddnsChallenge(body string) bool {
	l := strings.ToLower(body)
	for _, m := range []string{"captcha", "cf-challenge", "just a moment", "attention required", "cf-chl", "verify you are human", "g-recaptcha", "h-captcha"} {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

func (rapiddns) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	seen := map[string]bool{}
	for page := 1; page <= merklemapPages(s); page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		resp, err := s.HTTP.Get(ctx, rapiddnsBaseURL+"/subdomain/"+domain+"?page="+strconv.Itoa(page)+"&full=1",
			map[string]string{"Accept": "text/html"})
		if err != nil {
			var he *httpx.Error
			if errors.As(err, &he) {
				if he.StatusCode == 404 {
					return nil
				}
				if he.StatusCode == 403 {
					return fmt.Errorf("%w: rapiddns blocked the request (HTTP 403)", ErrUnavailable)
				}
			}
			return err
		}
		body := string(resp.Body)
		if rapiddnsChallenge(body) {
			return fmt.Errorf("%w: rapiddns served a CAPTCHA/challenge page; not bypassed", ErrUnavailable)
		}
		fresh := 0
		for _, n := range scope.FindInText(body, domain) {
			if !seen[n] {
				seen[n] = true
				fresh++
				emit(n)
			}
		}
		if fresh == 0 {
			return nil
		}
	}
	return nil
}

// Provider: Digitorus certificate details (https://certificatedetails.com).
// Docs: none published (no API contract); subfinder source is the only reference.
// Endpoint: GET https://certificatedetails.com/<domain> (public HTML page built from CT logs).
// Auth: none. Default: off (HTML scraping is fragile and CT data is covered by other sources).
// Parsing: scope-aware text extraction (scope.FindInText) over the page. If the site answers with a bot
// challenge (Cloudflare "Just a moment", CAPTCHA, HTTP 403/503) the source reports ErrUnavailable; no bypass is attempted.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not run for this release (site returned 403 to an automated fetch).
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/lunalully/lunatic/internal/scope"
)

var digitorusBaseURL = "https://certificatedetails.com"

type digitorus struct{}

func init() { Register(digitorus{}) }

func (digitorus) Info() Info {
	return Info{Name: "digitorus", URL: "https://certificatedetails.com", Auth: AuthNone, Default: false, RPS: 0.5, Burst: 1}
}

func digitorusChallenge(body string) bool {
	low := strings.ToLower(body)
	for _, m := range []string{"just a moment", "cf-chl", "challenge-platform", "captcha", "attention required", "verify you are human"} {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

func (digitorus) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	resp, err := s.HTTP.Get(ctx, digitorusBaseURL+"/"+url.PathEscape(domain), map[string]string{"Accept": "text/html"})
	if err != nil {
		if st := g2status(err); st == 403 || st == 503 {
			return fmt.Errorf("%w: certificatedetails.com refused the request (HTTP %d, likely bot protection); not bypassed", ErrUnavailable, st)
		}
		return err
	}
	body := string(resp.Body)
	if digitorusChallenge(body) {
		return fmt.Errorf("%w: certificatedetails.com served a bot challenge; not bypassed", ErrUnavailable)
	}
	for _, n := range scope.FindInText(body, domain) {
		emit(n)
	}
	return nil
}

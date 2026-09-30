// Package sources: sitedossier adapter.
//
// Provider:     Sitedossier (http://www.sitedossier.com)
// Docs:         none (no API; HTML pages). Behaviour follows subfinder.
// Endpoint:     GET {base}/parentdomain/{domain}, then the "next" links found
//
//	on each page (only same-host /parentdomain/ paths are used).
//
// Auth:         none. Default:false because the site is an HTML scraper with
//
//	anti-bot behaviour and no documented terms or limits.
//
// Pagination:   next-page link, bounded by Session.MaxPages.
// Blocking:     a CAPTCHA / "unusual traffic" page yields ErrUnavailable; the
//
//	adapter never tries to bypass it.
//
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/lunalully/lunatic/internal/scope"
)

var sitedossierBaseURL = "http://www.sitedossier.com"

var sitedossierNextRe = regexp.MustCompile(`<a href="(/parentdomain/[A-Za-z0-9./_-]+)"><b>`)

type sitedossier struct{}

func init() { Register(sitedossier{}) }

func (sitedossier) Info() Info {
	return Info{Name: "sitedossier", URL: "http://www.sitedossier.com", Auth: AuthNone, Default: false, RPS: 0.2, Burst: 1}
}

func sitedossierBlocked(body string) bool {
	l := strings.ToLower(body)
	for _, m := range []string{"captcha", "unusual traffic", "automated queries", "you have been blocked", "access denied"} {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

func (sitedossier) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	path := "/parentdomain/" + url.PathEscape(domain)
	visited := map[string]bool{}
	for page := 0; page < s.MaxPages && path != ""; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		visited[path] = true
		resp, err := s.HTTP.Get(ctx, sitedossierBaseURL+path, nil)
		if err != nil {
			return err
		}
		body := string(resp.Body)
		if sitedossierBlocked(body) {
			return fmt.Errorf("%w: sitedossier: anti-bot/CAPTCHA page served; not bypassing", ErrUnavailable)
		}
		for _, h := range scope.FindInText(body, domain) {
			emit(h)
		}
		path = ""
		for _, m := range sitedossierNextRe.FindAllStringSubmatch(body, -1) {
			if !visited[m[1]] {
				path = m[1]
				break
			}
		}
	}
	return nil
}

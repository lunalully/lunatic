// Package sources: waybackarchive adapter.
//
// Provider:     Internet Archive Wayback Machine (https://web.archive.org)
// Docs:         https://github.com/internetarchive/wayback/blob/master/wayback-cdx-server/README.md
// Endpoint:     GET {base}/cdx/search/cdx?url=*.{domain}&output=txt&fl=original
//
//	&collapse=urlkey&limit=50000&showResumeKey=true[&resumeKey=K]
//	ONLY the CDX index is read (one original URL per row). Archived
//	pages (/web/<ts>/...) and live sites are never fetched; only the
//	host part of each indexed URL is used.
//
// Auth:         none. Rate limits are undocumented (expect 429 / slow replies).
// Pagination:   resume key (a blank line, then the key, ends a txt response);
//
//	bounded by Session.MaxPages.
//
// Checked:      2026-09-29
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"net/url"
	"strings"
)

var waybackarchiveBaseURL = "https://web.archive.org"

const waybackarchivePageLimit = "50000"

type waybackarchive struct{}

func init() { Register(waybackarchive{}) }

func (waybackarchive) Info() Info {
	return Info{Name: "waybackarchive", URL: "https://web.archive.org", Auth: AuthNone, Default: true, RPS: 0.5, Burst: 1}
}

// waybackarchiveHost extracts the lowercase hostname from an indexed URL.
func waybackarchiveHost(raw string) string {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
	}
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if i := strings.LastIndex(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	for i := 0; i < 2; i++ { // some indexed hosts are percent-encoded (twice)
		if d, err := url.PathUnescape(s); err == nil {
			s = d
		}
	}
	if i := strings.IndexAny(s, "/?#:"); i >= 0 {
		s = s[:i]
	}
	return strings.Trim(strings.ToLower(s), ". \t*")
}

func (waybackarchive) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	suffix := "." + domain
	seen := map[string]bool{}
	resume := ""
	for page := 0; page < s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := waybackarchiveBaseURL + "/cdx/search/cdx?url=" + url.QueryEscape("*."+domain) +
			"&output=txt&fl=original&collapse=urlkey&limit=" + waybackarchivePageLimit + "&showResumeKey=true"
		if resume != "" {
			u += "&resumeKey=" + url.QueryEscape(resume)
		}
		resp, err := s.HTTP.Get(ctx, u, nil)
		if err != nil {
			return err
		}
		body := strings.ReplaceAll(string(resp.Body), "\r\n", "\n")
		rows, tail := body, ""
		if i := strings.Index(body, "\n\n"); i >= 0 {
			rows, tail = body[:i], strings.TrimSpace(body[i+2:])
		}
		for _, line := range strings.Split(rows, "\n") {
			h := waybackarchiveHost(line)
			if h != "" && strings.HasSuffix(h, suffix) && !seen[h] {
				seen[h] = true
				emit(h)
			}
		}
		if tail == "" || tail == resume || strings.ContainsAny(tail, " \n") {
			return nil
		}
		resume = tail
	}
	return nil
}

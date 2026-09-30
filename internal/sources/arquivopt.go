// Package sources: arquivopt adapter.
//
// Provider:     Arquivo.pt (Portuguese web archive, https://arquivo.pt)
// Docs:         https://github.com/arquivo/pwa-technologies/wiki/URL-search:-CDX-server-API
// Endpoint:     GET {base}/wayback/cdx?url=*.{domain}&output=json&fields=url&limit=N
//
//	Wildcard "*.domain" is the documented domain match (all subdomains);
//	output=json returns one JSON object per line; fields selects columns.
//	ONLY the CDX index is read. Archived pages and live sites are never
//	fetched; only the host part of each indexed URL is used.
//
// Auth:         none. Rate limits are not documented; RPS is kept low.
// Pagination:   the docs list none (only limit, max 100000), so exactly one
//
//	request is made per domain; MaxPages therefore cannot be exceeded.
//
// Yield:        low for domains outside Portugal (coverage is mostly .pt), so
//
//	the source is not in the default run (-s arquivopt or --all).
//
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

var arquivoptBaseURL = "https://arquivo.pt"

const arquivoptLimit = "100000"

type arquivopt struct{}

func init() { Register(arquivopt{}) }

func (arquivopt) Info() Info {
	return Info{Name: "arquivopt", URL: "https://arquivo.pt", Auth: AuthNone, Default: false, RPS: 0.5, Burst: 1}
}

func (arquivopt) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	u := arquivoptBaseURL + "/wayback/cdx?url=" + url.QueryEscape("*."+domain) + "&output=json&fields=url&limit=" + arquivoptLimit
	resp, err := s.HTTP.Get(ctx, u, nil)
	if err != nil {
		return err
	}
	suffix := "." + domain
	seen := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(resp.Body))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var row struct {
			URL string `json:"url"`
		}
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return fmt.Errorf("%w: arquivopt: invalid JSON line", ErrUnexpected)
		}
		h := waybackarchiveHost(row.URL)
		if h != "" && strings.HasSuffix(h, suffix) && !seen[h] {
			seen[h] = true
			emit(h)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%w: arquivopt: %v", ErrUnexpected, err)
	}
	return nil
}

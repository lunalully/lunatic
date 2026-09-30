// Provider: Common Crawl index server (CDX API).
// Docs: https://index.commoncrawl.org/ and https://commoncrawl.org/faq
// Endpoints: GET /collinfo.json (list of crawl indexes), then for each of the
// 3 newest indexes GET /<id>-index?url=*.<domain>&output=json&fl=url
// (&showNumPages=true, then &page=N). Only the index is queried; archived pages
// (WARC data) are never fetched. Auth: none. Heavily rate limited by the
// provider (HTTP 503 = slow down) so it is not in the default run.
// A 404 "No Captures found" for an index means zero results for that index.
// Pagination per index is bounded by Session.MaxPages.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release
// (run scripts/live-smoke.sh to check).
package sources

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/lunalully/lunatic/internal/httpx"
)

var commoncrawlBaseURL = "https://index.commoncrawl.org"

const commoncrawlIndexes = 3

var commoncrawlIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

type commoncrawl struct{}

func init() { Register(commoncrawl{}) }

func (commoncrawl) Info() Info {
	return Info{Name: "commoncrawl", URL: "https://commoncrawl.org", Auth: AuthNone, Default: false, RPS: 0.5, Burst: 1}
}

func commoncrawlNotFound(err error) bool {
	var he *httpx.Error
	return errors.As(err, &he) && he.StatusCode == 404
}

func (commoncrawl) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	var info []struct {
		ID string `json:"id"`
	}
	if err := s.HTTP.GetJSON(ctx, commoncrawlBaseURL+"/collinfo.json", nil, &info); err != nil {
		return err
	}
	var ids []string
	for _, i := range info {
		if commoncrawlIDRe.MatchString(i.ID) {
			ids = append(ids, i.ID)
		}
	}
	if len(ids) == 0 {
		return fmt.Errorf("%w: commoncrawl: collinfo.json lists no usable index", ErrUnexpected)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	if len(ids) > commoncrawlIndexes {
		ids = ids[:commoncrawlIndexes]
	}
	maxPages := s.MaxPages
	if maxPages <= 0 {
		maxPages = 10
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := commoncrawlIndex(ctx, domain, id, maxPages, s, seen, emit); err != nil {
			return err
		}
	}
	return nil
}

func commoncrawlIndex(ctx context.Context, domain, id string, maxPages int, s *Session, seen map[string]bool, emit func(string)) error {
	base := commoncrawlBaseURL + "/" + id + "-index?url=" + url.QueryEscape("*."+domain) + "&output=json&fl=url"
	// Ask how many pages exist.
	resp, err := s.HTTP.Get(ctx, base+"&showNumPages=true", nil)
	if err != nil {
		if commoncrawlNotFound(err) {
			return nil // "No Captures found"
		}
		return err
	}
	var np struct {
		Pages int `json:"pages"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(resp.Body), &np); err != nil {
		return fmt.Errorf("%w: commoncrawl: invalid showNumPages response: %v", ErrUnexpected, err)
	}
	pages := np.Pages
	if pages > maxPages {
		pages = maxPages
	}
	for p := 0; p < pages; p++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		resp, err := s.HTTP.Get(ctx, base+"&page="+strconv.Itoa(p), nil)
		if err != nil {
			if commoncrawlNotFound(err) {
				return nil
			}
			return err
		}
		sc := bufio.NewScanner(bytes.NewReader(resp.Body))
		sc.Buffer(make([]byte, 64*1024), 4<<20)
		for sc.Scan() {
			line := bytes.TrimSpace(sc.Bytes())
			if len(line) == 0 {
				continue
			}
			var rec struct {
				URL   string `json:"url"`
				Error string `json:"error"`
			}
			if err := json.Unmarshal(line, &rec); err != nil {
				return fmt.Errorf("%w: commoncrawl: invalid NDJSON line: %v", ErrUnexpected, err)
			}
			if rec.Error != "" {
				return fmt.Errorf("%w: commoncrawl: %s", ErrUnexpected, rec.Error)
			}
			if h := commoncrawlHost(rec.URL); h != "" && !seen[h] {
				seen[h] = true
				emit(h)
			}
		}
		if err := sc.Err(); err != nil {
			return fmt.Errorf("%w: commoncrawl: reading index page: %v", ErrUnexpected, err)
		}
	}
	return nil
}

// commoncrawlHost extracts only the hostname from an indexed URL.
func commoncrawlHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

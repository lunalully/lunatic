// Provider: DNSArchive (formerly DNSRepo), https://dnsarchive.net
// Docs: https://dnsarchive.net/api-access
// Endpoint: GET https://dnsarchive.net/api/?apikey=<KEY>&search=<domain>&page=N&limit=500
// Auth: required, two values: the API key (query parameter apikey, credential field "apikey") and the access token
// (header X-API-Access, credential field "token"). Paid plans only.
// Response: JSON array of records with a "domain" field (schema taken from third-party code; the provider publishes
// none). Pagination stops when a page is short, adds no new names (guards against an ignored page parameter) or at MaxPages.
// A JSON object response is treated as an error message.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var dnsrepoBaseURL = "https://dnsarchive.net"

const dnsrepoLimit = 500

type dnsrepo struct{}

func init() { Register(dnsrepo{}) }

func (dnsrepo) Info() Info {
	return Info{Name: "dnsrepo", URL: "https://dnsarchive.net", Auth: AuthRequired,
		CredFields: []string{"apikey", "token"}, Default: true, RPS: 1, Burst: 1}
}

func (dnsrepo) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	apikey, err := PickKey(s.Creds, "apikey")
	if err != nil {
		return err
	}
	token, err := PickKey(s.Creds, "token")
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for page := 1; page <= s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := dnsrepoBaseURL + "/api/?apikey=" + url.QueryEscape(apikey) + "&search=" + url.QueryEscape(domain) +
			"&page=" + strconv.Itoa(page) + "&limit=" + strconv.Itoa(dnsrepoLimit)
		resp, err := s.HTTP.Get(ctx, u, map[string]string{"X-API-Access": token, "Accept": "application/json"})
		if err != nil {
			return err
		}
		body := strings.TrimSpace(string(resp.Body))
		if body == "" || body == "null" {
			return nil
		}
		if strings.HasPrefix(body, "{") {
			var e struct {
				Error   string `json:"error"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(resp.Body, &e)
			msg := e.Error + e.Message
			low := strings.ToLower(msg)
			switch {
			case strings.Contains(low, "limit"), strings.Contains(low, "quota"):
				return fmt.Errorf("%w: dnsrepo: %.80s", ErrRateLimited, msg)
			case strings.Contains(low, "key"), strings.Contains(low, "token"), strings.Contains(low, "auth"):
				return fmt.Errorf("%w: dnsrepo: %.80s", ErrAuth, msg)
			}
			return fmt.Errorf("%w: dnsrepo error object: %.80s", ErrUnexpected, msg)
		}
		var rows []struct {
			Domain string `json:"domain"`
		}
		if err := json.Unmarshal(resp.Body, &rows); err != nil {
			return fmt.Errorf("%w: dnsrepo invalid JSON: %v", ErrUnexpected, err)
		}
		fresh := 0
		for _, r := range rows {
			n := strings.TrimSuffix(r.Domain, ".")
			if n == "" || seen[n] {
				continue
			}
			seen[n] = true
			fresh++
			emit(n)
		}
		if len(rows) < dnsrepoLimit || fresh == 0 {
			return nil
		}
	}
	return nil
}

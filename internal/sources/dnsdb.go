// Provider: DomainTools DNSDB (formerly Farsight), stored passive DNS.
// Docs: https://docs.domaintools.com/api/dnsdb/ (lookups: /lookups/rrset-lookups/, quotas: /rate-limits/).
// Endpoint: GET https://api.dnsdb.info/dnsdb/v2/lookup/rrset/name/*.<domain>?limit=N[&offset=M]
// Auth: required; header X-API-Key. Accept: application/x-ndjson.
// Format: streaming SAF (NDJSON): {"cond":"begin"}, {"obj":{"rrname":...}}..., terminated by
// {"cond":"succeeded"} or {"cond":"limited"} (more data: re-query with offset) or {"cond":"failed"}.
// A stream without a terminating cond is treated as truncated (ErrUnexpected). HTTP 404 means "no results".
// Only rrname is emitted. Paid product; quotas are per key.
// Date checked: 2026-09-29.
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var dnsdbBaseURL = "https://api.dnsdb.info"

const dnsdbLimit = 10000

type dnsdb struct{}

func init() { Register(dnsdb{}) }

func (dnsdb) Info() Info {
	return Info{Name: "dnsdb", URL: "https://www.domaintools.com/products/dnsdb/", Auth: AuthRequired,
		CredFields: []string{"api_key"}, Default: true, RPS: 1, Burst: 1}
}

func (dnsdb) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	headers := map[string]string{"X-API-Key": key, "Accept": "application/x-ndjson"}
	offset := 0
	for page := 0; page < s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		u := dnsdbBaseURL + "/dnsdb/v2/lookup/rrset/name/*." + url.PathEscape(domain) + "?limit=" + strconv.Itoa(dnsdbLimit)
		if offset > 0 {
			u += "&offset=" + strconv.Itoa(offset)
		}
		resp, err := s.HTTP.Get(ctx, u, headers)
		if err != nil {
			if g2status(err) == 404 {
				return nil
			}
			return err
		}
		names, cond, msg, perr := dnsdbParse(resp.Body)
		if perr != nil {
			return perr
		}
		for _, n := range names {
			emit(n)
		}
		switch cond {
		case "succeeded":
			return nil
		case "limited":
			if len(names) == 0 {
				return nil
			}
			offset += len(names)
		default:
			return fmt.Errorf("%w: dnsdb stream ended with cond %q %s", ErrUnexpected, cond, msg)
		}
	}
	return nil
}

// dnsdbParse reads a SAF stream and returns rrnames and the terminating cond.
func dnsdbParse(body []byte) (names []string, cond, msg string, err error) {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 64*1024), 8<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var rec struct {
			Cond string `json:"cond"`
			Msg  string `json:"msg"`
			Obj  *struct {
				Rrname string `json:"rrname"`
			} `json:"obj"`
		}
		if e := json.Unmarshal([]byte(line), &rec); e != nil {
			return nil, "", "", fmt.Errorf("%w: dnsdb invalid stream line", ErrUnexpected)
		}
		if rec.Obj != nil {
			if n := strings.TrimSuffix(rec.Obj.Rrname, "."); n != "" {
				names = append(names, n)
			}
			continue
		}
		switch rec.Cond {
		case "begin", "":
		case "failed":
			return nil, "", "", fmt.Errorf("%w: dnsdb query failed: %.100s", ErrUnexpected, rec.Msg)
		default:
			cond, msg = rec.Cond, rec.Msg
		}
	}
	if e := sc.Err(); e != nil {
		return nil, "", "", fmt.Errorf("%w: dnsdb stream: %v", ErrUnexpected, e)
	}
	return names, cond, msg, nil
}

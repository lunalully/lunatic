// Package sources: zoomeyeapi adapter.
//
// Provider:     ZoomEye (https://www.zoomeye.ai; also zoomeye.hk / zoomeye.org)
// Docs:         https://www.zoomeye.ai/doc (API v2; page text not readable in
//
//	detail when checked; body fields follow subfinder's client).
//
// Endpoint:     POST https://api.{host}/v2/search
//
//	body {"qbase64":base64(domain="D"),"page":N,"pagesize":1000,
//	      "fields":"domain","subtype":"web"}
//
// Auth:         header "API-KEY". The api_key credential may be given as
//
//	"KEY" (host defaults to zoomeye.ai). The regional host is the
//	optional credential field "host" (e.g. zoomeye.hk); for
//	backward compatibility, subfinder-style "zoomeye.hk:KEY" in
//	api_key is still accepted (an explicit host field wins).
//
// Pagination:   page until an empty page, a short page or page*pagesize>=total;
//
//	bounded by Session.MaxPages.
//
// Errors:       200 bodies carry code (60000 = ok) and message.
// Checked:      2026-09-29
// Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// zoomeyeapiBaseURL, when non-empty, replaces the per-host base (tests).
var zoomeyeapiBaseURL = ""

var zoomeyeapiPageSize = 1000

var zoomeyeapiHostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

type zoomeyeapi struct{}

func init() { Register(zoomeyeapi{}) }

func (zoomeyeapi) Info() Info {
	return Info{
		Name: "zoomeyeapi", URL: "https://www.zoomeye.ai",
		Auth: AuthRequired, CredFields: []string{"api_key"}, OptCredFields: []string{"host"},
		Default: false, RPS: 1, Burst: 1,
	}
}

// zoomeyeapiSplit separates an optional "host:" prefix from the key.
func zoomeyeapiSplit(v string) (host, key string) {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, ":"); i > 0 {
		h := strings.ToLower(v[:i])
		if zoomeyeapiHostRe.MatchString(h) && strings.Contains(h, ".") {
			return strings.TrimPrefix(h, "api."), v[i+1:]
		}
	}
	return "zoomeye.ai", v
}

func (zoomeyeapi) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	raw, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	host, key := zoomeyeapiSplit(raw)
	if h := strings.ToLower(strings.TrimSpace(s.Creds["host"])); h != "" {
		h = strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://"), "api.")
		h = strings.TrimSuffix(h, "/")
		if !zoomeyeapiHostRe.MatchString(h) {
			return fmt.Errorf("%w: zoomeyeapi host must be a bare host name", ErrUnexpected)
		}
		host = h
	}
	if key == "" {
		return fmt.Errorf("%w: api_key", ErrNoKey)
	}
	base := "https://api." + host
	if zoomeyeapiBaseURL != "" {
		base = zoomeyeapiBaseURL
	}
	hdr := map[string]string{"API-KEY": key}
	q := base64.StdEncoding.EncodeToString([]byte(`domain="` + domain + `"`))
	for page := 1; page <= s.MaxPages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{
			"qbase64": q, "page": page, "pagesize": zoomeyeapiPageSize,
			"fields": "domain", "subtype": "web",
		})
		var r struct {
			Code    json.Number `json:"code"`
			Message string      `json:"message"`
			Total   json.Number `json:"total"`
			Data    []struct {
				Domain string `json:"domain"`
			} `json:"data"`
		}
		if err := s.HTTP.PostJSON(ctx, base+"/v2/search", hdr, body, &r); err != nil {
			return err
		}
		if r.Code == "" {
			return fmt.Errorf("%w: zoomeyeapi: code missing", ErrUnexpected)
		}
		if r.Code.String() != "60000" {
			l := strings.ToLower(r.Message)
			switch {
			case strings.Contains(l, "key") || strings.Contains(l, "auth") || strings.Contains(l, "credential") || strings.Contains(l, "login"):
				return fmt.Errorf("%w: zoomeyeapi: %s", ErrAuth, r.Message)
			case strings.Contains(l, "limit") || strings.Contains(l, "quota") || strings.Contains(l, "frequen") || strings.Contains(l, "too many"):
				return fmt.Errorf("%w: zoomeyeapi: %s", ErrRateLimited, r.Message)
			}
			return fmt.Errorf("%w: zoomeyeapi: code %s: %s", ErrUnexpected, r.Code, r.Message)
		}
		for _, d := range r.Data {
			if d.Domain != "" {
				emit(d.Domain)
			}
		}
		total, _ := r.Total.Int64()
		if len(r.Data) < zoomeyeapiPageSize || (total > 0 && int64(page)*int64(zoomeyeapiPageSize) >= total) {
			return nil
		}
	}
	return nil
}

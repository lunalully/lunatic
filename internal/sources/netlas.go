// Package sources: netlas adapter.
//
// Provider: Netlas.io. Docs: https://docs.netlas.io/api-reference/
// Endpoints (indexed data only): GET https://app.netlas.io/api/domains_count/?q=...
// then POST https://app.netlas.io/api/domains/download/ with {"q","fields","source_type","size"}.
// Auth: "Authorization: Bearer <api_key>". Community plan is free but small
// (documented: 50 requests/day, download up to 200 results), hence netlasMaxSize.
// Checked: 2026-09-29. Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
// The download request body follows the Subfinder reference and is not confirmed in the official docs.
package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

var netlasBaseURL = "https://app.netlas.io"

// netlasMaxSize caps the download size (free-tier limit per the docs).
var netlasMaxSize = 200

type netlas struct{}

func init() { Register(netlas{}) }

func (netlas) Info() Info {
	return Info{
		Name: "netlas", URL: "https://netlas.io",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.5, Burst: 1,
	}
}

func (netlas) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"Authorization": "Bearer " + key, "Accept": "application/json"}
	q := "domain:*." + domain + " AND NOT domain:" + domain

	var cnt struct {
		Count   *int64 `json:"count"`
		Detail  any    `json:"detail"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := s.HTTP.GetJSON(ctx, netlasBaseURL+"/api/domains_count/?q="+url.QueryEscape(q), hdr, &cnt); err != nil {
		return err
	}
	if msg := netlasMsg(cnt.Error, cnt.Message, cnt.Detail); msg != "" && cnt.Count == nil {
		return merklemapClassify(msg)
	}
	if cnt.Count == nil {
		return fmt.Errorf("%w: netlas count response has no count field", ErrUnexpected)
	}
	if *cnt.Count <= 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	size := int64(netlasMaxSize)
	if *cnt.Count < size {
		size = *cnt.Count
	}
	body, _ := json.Marshal(map[string]any{"q": q, "fields": []string{"domain"}, "source_type": "include", "size": size})
	var raw json.RawMessage
	h := map[string]string{"Authorization": hdr["Authorization"], "Accept": "application/json"}
	if err := s.HTTP.PostJSON(ctx, netlasBaseURL+"/api/domains/download/", h, body, &raw); err != nil {
		return err
	}
	trim := bytes.TrimSpace(raw)
	if len(trim) > 0 && trim[0] == '{' {
		var e struct {
			Detail  any    `json:"detail"`
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(trim, &e)
		if msg := netlasMsg(e.Error, e.Message, e.Detail); msg != "" {
			return merklemapClassify(msg)
		}
		return fmt.Errorf("%w: netlas download returned an object, expected an array", ErrUnexpected)
	}
	var items []struct {
		Data struct {
			Domain string `json:"domain"`
		} `json:"data"`
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal(trim, &items); err != nil {
		return fmt.Errorf("%w: netlas download: %v", ErrUnexpected, err)
	}
	for _, it := range items {
		if it.Data.Domain != "" {
			emit(it.Data.Domain)
		} else if it.Domain != "" {
			emit(it.Domain)
		}
	}
	return nil
}

func netlasMsg(errStr, msg string, detail any) string {
	if errStr != "" {
		return errStr
	}
	if msg != "" {
		return msg
	}
	if d, ok := detail.(string); ok {
		return d
	}
	if detail != nil {
		b, _ := json.Marshal(detail)
		return string(b)
	}
	return ""
}

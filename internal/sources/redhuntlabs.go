// Package sources: redhuntlabs adapter.
//
// Provider: RedHunt Labs Attack Surface Recon API. Docs: https://redhuntlabs.com/attack-surface-recon-api/
// Endpoint: GET https://reconapi.redhuntlabs.com/community/v1/domains/subdomains?domain=<d>&page=<n>&page_size=<n>
// (community plan prefix; other plans use a different prefix, not configurable here).
// Auth: "X-BLOBR-KEY: <api_key>". Community plan: 100 requests/month, so at most
// redhuntlabsMaxPages (3) requests are made regardless of MaxPages.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from build environment.
package sources

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

var redhuntlabsBaseURL = "https://reconapi.redhuntlabs.com"

var (
	redhuntlabsMaxPages = 3
	redhuntlabsPageSize = 1000
)

type redhuntlabs struct{}

func init() { Register(redhuntlabs{}) }

func (redhuntlabs) Info() Info {
	return Info{
		Name: "redhuntlabs", URL: "https://redhuntlabs.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 1, Burst: 1,
	}
}

func (redhuntlabs) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	hdr := map[string]string{"X-BLOBR-KEY": key, "Accept": "application/json"}
	pages := merklemapPages(s)
	if pages > redhuntlabsMaxPages {
		pages = redhuntlabsMaxPages
	}
	for page := 1; page <= pages; page++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var resp struct {
			Subdomains *[]string `json:"subdomains"`
			Error      string    `json:"error"`
			Message    string    `json:"message"`
			Metadata   struct {
				ResultCount int `json:"result_count"`
				PageSize    int `json:"page_size"`
			} `json:"metadata"`
		}
		u := redhuntlabsBaseURL + "/community/v1/domains/subdomains?domain=" + url.QueryEscape(domain) +
			"&page=" + strconv.Itoa(page) + "&page_size=" + strconv.Itoa(redhuntlabsPageSize)
		if err := s.HTTP.GetJSON(ctx, u, hdr, &resp); err != nil {
			return err
		}
		if resp.Subdomains == nil {
			if m := resp.Error + resp.Message; m != "" {
				return merklemapClassify(m)
			}
			return fmt.Errorf("%w: redhuntlabs response has no subdomains field", ErrUnexpected)
		}
		if len(*resp.Subdomains) == 0 {
			return nil
		}
		for _, h := range *resp.Subdomains {
			emit(h)
		}
		size := resp.Metadata.PageSize
		if size <= 0 {
			size = redhuntlabsPageSize
		}
		if page*size >= resp.Metadata.ResultCount {
			return nil
		}
	}
	return nil
}

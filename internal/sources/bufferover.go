// Provider: BufferOver TLS (tls.bufferover.run, Run.Scans).
// Docs: https://newtls.bufferover.run/ . Endpoint: GET /dns?q=.<domain>
// Auth: required, header x-api-key (free tier 100 requests/month, non-commercial).
// Reliability: the service has had prolonged outages (subfinder discussion #1739);
// treat failures as soft. The exact result string format is not confirmed, so
// hostnames are extracted defensively from every FDNS_A/RDNS/Results entry and
// IPs, hashes and organisation names are dropped.
// Checked: 2026-09-29. Verification: fixture only; live test not possible from
// build environment (egress blocked).
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/lunalully/lunatic/internal/scope"
)

var bufferoverBaseURL = "https://tls.bufferover.run"

type bufferover struct{}

func init() { Register(bufferover{}) }

func (bufferover) Info() Info {
	return Info{
		Name: "bufferover", URL: "https://tls.bufferover.run",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Default: true, RPS: 0.5, Burst: 1, // runner skips it when no key is configured
	}
}

func (bufferover) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	key, err := PickKey(s.Creds, "api_key")
	if err != nil {
		return err
	}
	var r struct {
		Meta struct {
			Errors []json.RawMessage `json:"Errors"`
		} `json:"Meta"`
		Message string            `json:"message"`
		Error   string            `json:"error"`
		FDNSA   []json.RawMessage `json:"FDNS_A"`
		RDNS    []json.RawMessage `json:"RDNS"`
		Results []json.RawMessage `json:"Results"`
	}
	u := bufferoverBaseURL + "/dns?q=" + url.QueryEscape("."+domain)
	if err := s.HTTP.GetJSON(ctx, u, map[string]string{"x-api-key": key, "Accept": "application/json"}, &r); err != nil {
		return err
	}
	if len(r.Meta.Errors) > 0 || r.Error != "" {
		msg := strings.TrimSpace(r.Error)
		if len(r.Meta.Errors) > 0 {
			msg = string(r.Meta.Errors[0])
		}
		return bufferoverProviderErr(msg)
	}
	if len(r.FDNSA)+len(r.RDNS)+len(r.Results) == 0 && r.Message != "" {
		return bufferoverProviderErr(r.Message)
	}
	for _, list := range [][]json.RawMessage{r.FDNSA, r.RDNS, r.Results} {
		for _, e := range list {
			for _, n := range scope.Extract(string(e), domain) {
				emit(n)
			}
		}
	}
	return nil
}

func bufferoverProviderErr(msg string) error {
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "api key") || strings.Contains(l, "forbidden") || strings.Contains(l, "unauthor"):
		return fmt.Errorf("%w: bufferover: %s", ErrAuth, msg)
	case strings.Contains(l, "limit") || strings.Contains(l, "quota") || strings.Contains(l, "too many"):
		return fmt.Errorf("%w: bufferover: %s", ErrRateLimited, msg)
	}
	return fmt.Errorf("%w: bufferover: %s", ErrUnexpected, msg)
}

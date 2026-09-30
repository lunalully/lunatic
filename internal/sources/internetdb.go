// Package sources: internetdb adapter (phase 2).
//
// Provider:     Shodan InternetDB (https://internetdb.shodan.io)
// Docs:         https://internetdb.shodan.io (free, no key, non-commercial use)
// Endpoint:     GET {base}/{ip} -> JSON {"hostnames":[...],"ip":...,"ports":[...],...}
//
//	The API is IP-based only. Lunatic never resolves DNS, so this is a
//	phase-2 source (Info.Phase2): it only uses the public IPs that phase-1
//	providers already returned (Session.IPs). Each IP is sent to
//	internetdb.shodan.io as a path parameter and is NEVER contacted.
//	Only "hostnames" is read; the runner scope-filters them.
//
// Limits:       at most internetdbMaxIPs addresses per domain (one request each).
// Errors:       404 means no data for that IP (skipped). Zero IPs = success, no request.
// Auth:         none. RPS 1.
// Checked:      2026-09-29
// Verification: fixture only; live test not run for this release; run scripts/live-smoke.sh to check.
package sources

import (
	"context"
	"net/netip"
	"net/url"
)

var internetdbBaseURL = "https://internetdb.shodan.io"

const internetdbMaxIPs = 50

type internetdb struct{}

func init() { Register(internetdb{}) }

func (internetdb) Info() Info {
	return Info{Name: "internetdb", URL: "https://internetdb.shodan.io", Auth: AuthNone, Default: true, RPS: 1, Burst: 1, Phase2: true}
}

func (internetdb) Enumerate(ctx context.Context, domain string, s *Session, emit func(string)) error {
	n := 0
	for _, ip := range s.IPs {
		if n >= internetdbMaxIPs {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		n++
		var r struct {
			Hostnames []string `json:"hostnames"`
		}
		if err := s.HTTP.GetJSON(ctx, internetdbBaseURL+"/"+url.PathEscape(addr.String()), nil, &r); err != nil {
			if g2status(err) == 404 {
				continue
			}
			return err
		}
		for _, h := range r.Hostnames {
			emit(h)
		}
	}
	return nil
}

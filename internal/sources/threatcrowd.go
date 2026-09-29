// Package sources: threatcrowd adapter (DISABLED).
//
// Provider:     ThreatCrowd (https://www.threatcrowd.org)
// Docs:         https://threatcrowd.blogspot.com/p/api.html
// Status:       registered for the coverage matrix only; never contacts the
//
//	network. The provider's own API page says the API "may be
//	withdrawn, and is likely to be flaky" (limit 1 request/10 s),
//	the service is unmaintained, it is served over plain HTTP in
//	known clients, and its data is redundant with AlienVault OTX.
//
// Checked:      2026-09-29
// Verification: n/a (disabled).
package sources

import "context"

type threatcrowd struct{}

func init() { Register(threatcrowd{}) }

func (threatcrowd) Info() Info {
	return Info{
		Name: "threatcrowd", URL: "https://www.threatcrowd.org", Auth: AuthNone,
		Disabled: true,
		DisabledReason: "unmaintained: the provider's API page states it may be withdrawn and is likely flaky " +
			"(1 request/10 s, plain-HTTP host in known clients); data is redundant with AlienVault OTX",
	}
}

func (threatcrowd) Enumerate(context.Context, string, *Session, func(string)) error {
	return ErrDisabled
}

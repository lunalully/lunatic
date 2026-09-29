// Package sources: robtex (disabled).
//
// Provider: Robtex. Docs: https://www.robtex.com/api/
// Checked: 2026-09-29. Verification: not implemented, no endpoint is called.
package sources

import "context"

type robtex struct{}

func init() { Register(robtex{}) }

func (robtex) Info() Info {
	return Info{
		Name: "robtex", URL: "https://www.robtex.com/api/",
		Auth: AuthNone, Disabled: true,
		DisabledReason: "the free Robtex API (10 requests/hour) only returns passive-DNS records owned by the queried name, not a subdomain list; subdomain discovery needs the paid Pro API plus one reverse lookup per IP",
	}
}

func (robtex) Enumerate(context.Context, string, *Session, func(string)) error { return ErrDisabled }

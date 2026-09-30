// Provider: Domains Project (https://domainsproject.org).
// Docs: none public. The API (api.domainsproject.org) is reported to be approval/subscription based and was
// unreachable (HTTP 502) when this adapter was written; its shape is known only from third-party code.
// Status: DISABLED. No endpoint is implemented.
// Date checked: 2026-09-29. Verification: n/a (disabled).
package sources

import "context"

type domainsproject struct{}

func init() { Register(domainsproject{}) }

func (domainsproject) Info() Info {
	return Info{
		Name: "domainsproject", URL: "https://domainsproject.org", Auth: AuthRequired, CredFields: []string{"username", "password"},
		Disabled:       true,
		DisabledReason: "undocumented API available by approval only; no public documentation or stable endpoint",
	}
}

func (domainsproject) Enumerate(context.Context, string, *Session, func(string)) error {
	return ErrDisabled
}

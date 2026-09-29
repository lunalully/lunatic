// Provider: Censys Platform (https://platform.censys.io, docs https://docs.censys.com).
// DISABLED. The legacy Search v2 API (id:secret) was replaced by the Platform
// API (Bearer personal access token, optional X-Organization-ID). The
// certificate-search request/response schema (query field names, result path for
// certificate names) could not be confirmed from official documentation, only
// from third-party client code, and free accounts are limited to lookup
// endpoints while each search costs credits. No endpoint is called.
// Checked: 2026-09-29. Verification: none (disabled).
package sources

import "context"

type censys struct{}

func init() { Register(censys{}) }

func (censys) Info() Info {
	return Info{
		Name: "censys", URL: "https://platform.censys.io",
		Auth: AuthRequired, CredFields: []string{"api_token", "organization_id"},
		Disabled: true,
		DisabledReason: "Censys Platform API certificate search schema not confirmed in official docs, " +
			"and free accounts cannot use search endpoints (credit-metered); legacy id:secret API is retired",
		RPS: 0.5, Burst: 1,
	}
}

func (censys) Enumerate(context.Context, string, *Session, func(string)) error { return ErrDisabled }

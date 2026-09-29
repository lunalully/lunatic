// Provider: Chinaz API (https://api.chinaz.com).
// DISABLED. The only documented endpoint that yields host names is the Alexa
// rank API (ContributingSubdomainList), built on data from the retired Alexa
// service; the current official docs do not confirm a subdomain-listing
// endpoint. No endpoint is called.
// Checked: 2026-09-29. Verification: none (disabled).
package sources

import "context"

type chinaz struct{}

func init() { Register(chinaz{}) }

func (chinaz) Info() Info {
	return Info{
		Name: "chinaz", URL: "https://api.chinaz.com",
		Auth: AuthRequired, CredFields: []string{"api_key"},
		Disabled:       true,
		DisabledReason: "documented API returns Alexa/rank data; subdomain listing not confirmed in current official docs",
		RPS:            0.5, Burst: 1,
	}
}

func (chinaz) Enumerate(context.Context, string, *Session, func(string)) error { return ErrDisabled }

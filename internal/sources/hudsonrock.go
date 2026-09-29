// Provider: Hudson Rock (https://www.hudsonrock.com).
// Status: DISABLED by policy. The dataset behind its free OSINT endpoints is derived from infostealer malware
// infections (stolen employee/customer credential telemetry). Lunatic does not process infostealer-derived data.
// No endpoint is implemented.
// Date checked: 2026-09-29. Verification: n/a (disabled).
package sources

import "context"

type hudsonrock struct{}

func init() { Register(hudsonrock{}) }

func (hudsonrock) Info() Info {
	return Info{
		Name: "hudsonrock", URL: "https://www.hudsonrock.com", Auth: AuthNone,
		Disabled:       true,
		DisabledReason: "dataset derives from infostealer infections (stolen-credential telemetry); Lunatic does not process infostealer-derived data for ethical and legal reasons",
	}
}

func (hudsonrock) Enumerate(context.Context, string, *Session, func(string)) error {
	return ErrDisabled
}

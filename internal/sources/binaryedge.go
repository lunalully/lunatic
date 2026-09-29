// Package sources: binaryedge adapter (DISABLED).
//
// Provider:     BinaryEdge (https://www.binaryedge.io)
// Status:       the service and its API were shut down on 2025-03-31
//
//	(BinaryEdge customers were transitioned to Coalition Control).
//	Registered only so the coverage matrix stays complete; it
//	never contacts the network.
//
// Checked:      2026-09-29
// Verification: n/a (disabled).
package sources

import "context"

type binaryedge struct{}

func init() { Register(binaryedge{}) }

func (binaryedge) Info() Info {
	return Info{
		Name: "binaryedge", URL: "https://www.binaryedge.io", Auth: AuthNone,
		Disabled:       true,
		DisabledReason: "service shut down on 2025-03-31 (BinaryEdge API/accounts ended; customers moved to Coalition Control)",
	}
}

func (binaryedge) Enumerate(context.Context, string, *Session, func(string)) error {
	return ErrDisabled
}

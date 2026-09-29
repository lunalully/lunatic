// Package errs holds the typed error values shared by httpx, sources and the
// runner. It exists separately to avoid import cycles; the sources package
// re-exports every value so adapters only need to import sources.
package errs

import (
	"context"
	"errors"
)

var (
	ErrNoKey       = errors.New("missing credentials")
	ErrAuth        = errors.New("authentication failed")
	ErrRateLimited = errors.New("rate limited")
	ErrTimeout     = errors.New("timeout")
	ErrUnexpected  = errors.New("unexpected response")
	ErrUnavailable = errors.New("service unavailable")
	ErrBlockedHost = errors.New("blocked host (passive guard)")
	ErrDisabled    = errors.New("source disabled")
)

// Kind maps an error to a short stable identifier used in summaries:
// no_key, auth, rate_limited, timeout, unexpected, unavailable, blocked,
// disabled, canceled.
func Kind(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, ErrNoKey):
		return "no_key"
	case errors.Is(err, ErrAuth):
		return "auth"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrBlockedHost):
		return "blocked"
	case errors.Is(err, ErrDisabled):
		return "disabled"
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrUnavailable):
		return "unavailable"
	case errors.Is(err, context.Canceled):
		return "canceled"
	default:
		return "unexpected"
	}
}

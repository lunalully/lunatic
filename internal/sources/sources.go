// Package sources defines the contract every provider adapter implements and
// the registry adapters join from their init() functions.
//
// One adapter = one file internal/sources/<name>.go. See docs/DEVELOPMENT.md.
package sources

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/lunalully/lunatic/internal/errs"
	"github.com/lunalully/lunatic/internal/httpx"
)

// Typed errors, re-exported from internal/errs (same values, so errors.Is works
// across packages). Adapters return these (directly or wrapped with %w);
// httpx already returns them for HTTP failures.
var (
	ErrNoKey       = errs.ErrNoKey
	ErrAuth        = errs.ErrAuth
	ErrRateLimited = errs.ErrRateLimited
	ErrTimeout     = errs.ErrTimeout
	ErrUnexpected  = errs.ErrUnexpected
	ErrUnavailable = errs.ErrUnavailable
	ErrBlockedHost = errs.ErrBlockedHost
	ErrDisabled    = errs.ErrDisabled
)

// AuthType says whether a source needs credentials.
type AuthType int

const (
	AuthNone     AuthType = iota // works without credentials
	AuthRequired                 // unusable without every CredField
	AuthOptional                 // works without credentials; credentials improve results
)

func (a AuthType) String() string {
	switch a {
	case AuthRequired:
		return "required"
	case AuthOptional:
		return "optional"
	}
	return "none"
}

// Info describes a source.
type Info struct {
	Name           string // identifier, lowercase, e.g. "crtsh"
	URL            string // official site
	Auth           AuthType
	CredFields     []string // required credentials, e.g. {"api_key"} or {"api_id","api_secret"}; empty for AuthNone
	OptCredFields  []string // optional credentials (loaded like CredFields but never required for readiness)
	Default        bool     // in the default run (no -s/--all) when usable; AuthRequired+Default = only when credentials are configured
	Disabled       bool     // registered only for listing/coverage; never run
	DisabledReason string
	RPS            float64 // provider rate limit (requests/second) for httpx; 0 = unlimited
	Burst          int     // token bucket burst (default 1)
	// Phase2 sources run after every phase-1 source has finished and receive
	// the public IP addresses phase-1 providers already returned (Session.IPs).
	// They never resolve DNS and never contact those IPs.
	Phase2 bool
}

// AllCredFields returns CredFields followed by OptCredFields.
func (i Info) AllCredFields() []string {
	out := make([]string, 0, len(i.CredFields)+len(i.OptCredFields))
	out = append(out, i.CredFields...)
	return append(out, i.OptCredFields...)
}

// Session is what the runner hands to Enumerate.
type Session struct {
	// HTTP is the ONLY HTTP client an adapter may use.
	HTTP *httpx.Client
	// Creds holds this source's credentials (field -> value); only the fields
	// listed in Info.CredFields and Info.OptCredFields, only non-empty ones.
	Creds map[string]string
	// Log is a verbose logger. The runner redacts secrets before printing, but
	// adapters must still never log credentials. May be nil-safe: use s.Logf.
	Log func(format string, args ...any)
	// MaxPages is the maximum number of paginated requests (default 10);
	// adapters must stop at this.
	MaxPages int
	// ReportIP is set by the runner for phase-1 sources. Adapters call s.IP for
	// an address their provider already returned; no extra request is made.
	ReportIP func(ip string)
	// IPs holds the unique public addresses phase-1 sources reported for the
	// current domain (sorted). Only filled for Info.Phase2 sources. They may
	// only be sent to a provider as query data, never contacted.
	IPs []string
}

// IP reports an IP address received from the provider (nil-safe). The runner
// keeps only unique public addresses.
func (s *Session) IP(ip string) {
	if s != nil && s.ReportIP != nil {
		s.ReportIP(strings.TrimSpace(ip))
	}
}

// Logf logs through Log if set.
func (s *Session) Logf(format string, args ...any) {
	if s != nil && s.Log != nil {
		s.Log(format, args...)
	}
}

// Source is a passive provider adapter.
type Source interface {
	Info() Info
	// Enumerate queries ONLY the provider's query/search endpoints and calls
	// emit for each raw candidate name (the runner normalizes and scopes them).
	// It returns nil on success (even with zero results) or a typed error.
	Enumerate(ctx context.Context, domain string, s *Session, emit func(raw string)) error
}

var (
	mu       sync.RWMutex
	registry = map[string]Source{}
)

// Register adds a source; call it from an adapter's init(). Panics on a
// duplicate or empty name.
func Register(s Source) {
	name := s.Info().Name
	if name == "" {
		panic("sources: Register with empty name")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("sources: duplicate source %q", name))
	}
	registry[name] = s
}

// All returns every registered source sorted by name.
func All() []Source {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Source, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Info().Name < out[j].Info().Name })
	return out
}

// Get returns the named source.
func Get(name string) (Source, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := registry[name]
	return s, ok
}

// PickKey returns creds[field] or an error wrapping ErrNoKey when empty.
func PickKey(creds map[string]string, field string) (string, error) {
	if v := creds[field]; v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%w: %s", ErrNoKey, field)
}

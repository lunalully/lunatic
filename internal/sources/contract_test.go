package sources

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/httpx"
)

type countingTransport struct{ n int32 }

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	atomic.AddInt32(&c.n, 1)
	return nil, errors.New("network disabled in contract test")
}

func TestContractRegistry(t *testing.T) {
	all := All()
	if len(all) != 54 {
		t.Errorf("registered sources = %d, want 54", len(all))
	}
	nameRe := regexp.MustCompile(`^[a-z0-9]+$`)
	for _, src := range all {
		i := src.Info()
		if !nameRe.MatchString(i.Name) || i.URL == "" {
			t.Errorf("%s: bad name or empty URL", i.Name)
		}
		if i.Auth == AuthRequired && len(i.CredFields) == 0 {
			t.Errorf("%s: AuthRequired without CredFields", i.Name)
		}
		if i.Auth == AuthNone && (len(i.CredFields) > 0 || len(i.OptCredFields) > 0) {
			t.Errorf("%s: AuthNone with credential fields", i.Name)
		}
		if i.Disabled && i.DisabledReason == "" {
			t.Errorf("%s: disabled without reason", i.Name)
		}
	}
}

// Disabled sources return ErrDisabled; keyed sources return ErrNoKey without
// touching the network; nothing panics on a nil Creds map.
func TestContractNoKeyAndDisabled(t *testing.T) {
	for _, src := range All() {
		i := src.Info()
		ct := &countingTransport{}
		s := &Session{HTTP: httpx.New(httpx.Options{SourceName: i.Name, TargetDomain: "example.com", Transport: ct, MaxRetries: -1, BackoffBase: time.Millisecond}), MaxPages: 2}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := src.Enumerate(ctx, "example.com", s, func(string) {})
		cancel()
		switch {
		case i.Disabled:
			if !errors.Is(err, ErrDisabled) || ct.n != 0 {
				t.Errorf("%s: disabled source: err=%v requests=%d", i.Name, err, ct.n)
			}
		case i.Auth == AuthRequired:
			if !errors.Is(err, ErrNoKey) || ct.n != 0 {
				t.Errorf("%s: keyed source without creds: err=%v requests=%d", i.Name, err, ct.n)
			}
		}
	}
}

// Static rules: adapters never use net/http or raw sockets/DNS; every adapter
// that talks to the network has a BaseURL variable.
func TestContractSourceFiles(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	bad := regexp.MustCompile(`"net/http"|net\.Dial|net\.Lookup|net\.Resolver|http\.(Get|Post|Client|DefaultClient|NewRequest)|os/exec`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		src := string(b)
		if bad.MatchString(src) {
			t.Errorf("%s: uses forbidden network primitive (%s)", f, bad.FindString(src))
		}
		if strings.Contains(src, "Register(") && f != "sources.go" && !strings.Contains(src, "Disabled:") && !regexp.MustCompile(`var \w+BaseURL\b`).MatchString(src) {
			t.Errorf("%s: no BaseURL variable", f)
		}
	}
}

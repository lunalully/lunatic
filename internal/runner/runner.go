// Package runner executes sources concurrently, normalizes and dedupes their
// output and records per-source outcomes.
package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lunalully/lunatic/internal/config"
	"github.com/lunalully/lunatic/internal/errs"
	"github.com/lunalully/lunatic/internal/httpx"
	"github.com/lunalully/lunatic/internal/scope"
	"github.com/lunalully/lunatic/internal/sources"
)

// Status is the class of a per-source outcome.
type Status string

const (
	StatusOK      Status = "ok"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
)

// Task is one source to run with its credentials.
type Task struct {
	Source sources.Source
	Creds  map[string]string // usable creds for this source only
}

// Options tunes a run. Zero values give the documented defaults.
type Options struct {
	Concurrency   int                              // sources at once (default 10)
	SourceTimeout time.Duration                    // per source AND domain enumeration (default 90s)
	MaxPages      int                              // default 10
	Verbose       func(format string, args ...any) // already-redacted lines; may be called concurrently
	Transport     http.RoundTripper                // test hook, passed to httpx
}

// Outcome is what happened to one source.
type Outcome struct {
	Source   string
	Status   Status
	Kind     string // failed: no_key, auth, rate_limited, timeout, unexpected, unavailable, blocked, canceled
	Message  string // skipped reason or redacted failure message
	Count    int    // unique in-scope subdomains this source contributed
	Duration time.Duration
}

// Finding is one subdomain with provenance.
type Finding struct {
	Domain    string
	Subdomain string
	Sources   []string // sorted
}

// Results is the deterministic outcome of a run: Findings ordered by input
// domain order then subdomain; Outcomes sorted by source name.
type Results struct {
	Findings []Finding
	Outcomes []Outcome
}

// Summary counts outcomes and unique subdomains.
type Summary struct {
	OK, Skipped, Failed int
	Total               int // unique subdomain names across all domains
}

// Summary computes the counters.
func (r *Results) Summary() Summary {
	var s Summary
	for _, o := range r.Outcomes {
		switch o.Status {
		case StatusOK:
			s.OK++
		case StatusSkipped:
			s.Skipped++
		default:
			s.Failed++
		}
	}
	seen := map[string]bool{}
	for _, f := range r.Findings {
		seen[f.Subdomain] = true
	}
	s.Total = len(seen)
	return s
}

// SkipReason returns "" if the source can run with creds, otherwise why it
// is skipped (disabled, or missing credentials with the env var names).
func SkipReason(info sources.Info, creds map[string]string) string {
	if info.Disabled {
		if info.DisabledReason != "" {
			return info.DisabledReason
		}
		return "disabled"
	}
	if info.Auth == sources.AuthRequired {
		var missing []string
		for _, f := range info.CredFields {
			if creds[f] == "" {
				missing = append(missing, config.EnvName(info.Name, f))
			}
		}
		if len(missing) > 0 {
			return "missing credentials: " + strings.Join(missing, ", ")
		}
	}
	return ""
}

type collector struct {
	mu   sync.Mutex
	prov map[string]map[string]map[string]bool // domain -> sub -> sources
}

func (c *collector) add(domain, sub, source string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.prov[domain]
	if m == nil {
		m = map[string]map[string]bool{}
		c.prov[domain] = m
	}
	s := m[sub]
	if s == nil {
		s = map[string]bool{}
		m[sub] = s
	}
	fresh := !s[source]
	s[source] = true
	return fresh
}

// Run executes tasks against domains (canonical, see scope.ParseDomain). It
// always returns Results; interruption is visible as outcomes of kind
// "canceled" and ctx.Err().
func Run(ctx context.Context, domains []string, tasks []Task, opts Options) *Results {
	if opts.Concurrency < 1 {
		opts.Concurrency = 10
	}
	if opts.SourceTimeout <= 0 {
		opts.SourceTimeout = 90 * time.Second
	}
	if opts.MaxPages < 1 {
		opts.MaxPages = 10
	}
	col := &collector{prov: map[string]map[string]map[string]bool{}}
	outcomes := make([]Outcome, len(tasks))

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < opts.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				outcomes[i] = runOne(ctx, domains, tasks[i], opts, col)
			}
		}()
	}
	for i := range tasks {
		if reason := SkipReason(tasks[i].Source.Info(), tasks[i].Creds); reason != "" {
			outcomes[i] = Outcome{Source: tasks[i].Source.Info().Name, Status: StatusSkipped, Message: reason}
			continue
		}
		select {
		case jobs <- i:
		case <-ctx.Done():
			outcomes[i] = Outcome{Source: tasks[i].Source.Info().Name, Status: StatusFailed, Kind: "canceled", Message: "not started: canceled"}
		}
	}
	close(jobs)
	wg.Wait()

	res := &Results{Outcomes: outcomes}
	sort.Slice(res.Outcomes, func(i, j int) bool { return res.Outcomes[i].Source < res.Outcomes[j].Source })
	for _, d := range domains {
		subs := make([]string, 0, len(col.prov[d]))
		for s := range col.prov[d] {
			subs = append(subs, s)
		}
		sort.Strings(subs)
		for _, s := range subs {
			srcs := make([]string, 0, len(col.prov[d][s]))
			for n := range col.prov[d][s] {
				srcs = append(srcs, n)
			}
			sort.Strings(srcs)
			res.Findings = append(res.Findings, Finding{Domain: d, Subdomain: s, Sources: srcs})
		}
	}
	return res
}

func runOne(ctx context.Context, domains []string, t Task, opts Options, col *collector) (out Outcome) {
	info := t.Source.Info()
	out = Outcome{Source: info.Name, Status: StatusOK}
	start := time.Now()
	defer func() { out.Duration = time.Since(start) }()

	var secrets []string
	for _, v := range t.Creds {
		secrets = append(secrets, v)
		// composite values such as "host:KEY" (zoomeyeapi): also mask the parts.
		if strings.Contains(v, ":") {
			for _, part := range strings.Split(v, ":") {
				if len(part) >= 8 {
					secrets = append(secrets, part)
				}
			}
		}
	}
	logf := func(format string, args ...any) {
		if opts.Verbose != nil {
			opts.Verbose("%s", httpx.Redact(fmt.Sprintf(format, args...), secrets))
		}
	}
	client := httpx.New(httpx.Options{
		SourceName:        info.Name,
		RequestsPerSecond: info.RPS,
		Burst:             info.Burst,
		TargetDomain:      domains[0],
		TargetDomains:     domains,
		Secrets:           secrets,
		Verbose:           logf,
		Transport:         opts.Transport,
	})
	sess := &sources.Session{
		HTTP:     client,
		Creds:    t.Creds,
		Log:      func(f string, a ...any) { logf("[%s] "+f, append([]any{info.Name}, a...)...) },
		MaxPages: opts.MaxPages,
	}

	own := map[string]bool{}
	var firstErr error
	for _, d := range domains {
		if err := ctx.Err(); err != nil {
			firstErr = err
			break
		}
		domain := d
		err := enumerate(ctx, opts.SourceTimeout, t.Source, domain, sess, func(raw string) {
			if n, ok := scope.Normalize(raw, domain); ok {
				col.add(domain, n, info.Name)
				own[domain+"\x00"+n] = true
			}
		})
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	out.Count = len(own)
	if firstErr == nil {
		return out
	}
	kind := errs.Kind(firstErr)
	msg := client.Redact(firstErr.Error())
	if kind == "disabled" {
		out.Status, out.Message = StatusSkipped, msg
		return out
	}
	out.Status, out.Kind, out.Message = StatusFailed, kind, msg
	return out
}

// enumerate runs one Enumerate call with a timeout, converting panics to
// errors and serializing emit so adapters may call it from several goroutines.
func enumerate(ctx context.Context, timeout time.Duration, src sources.Source, domain string, sess *sources.Session, emit func(string)) (err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: panic: %v", errs.ErrUnexpected, r)
		}
	}()
	var mu sync.Mutex
	err = src.Enumerate(ctx, domain, sess, func(raw string) {
		mu.Lock()
		defer mu.Unlock()
		emit(raw)
	})
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) && !errors.Is(err, context.Canceled) && !errors.Is(err, errs.ErrTimeout) {
		err = fmt.Errorf("%w: %v", errs.ErrTimeout, err) // adapter failed because our deadline hit
	}
	return err
}

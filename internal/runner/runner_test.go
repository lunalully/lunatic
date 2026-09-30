package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lunalully/lunatic/internal/sources"
)

// fake is the offline test source used across runner tests.
type fake struct {
	info sources.Info
	fn   func(ctx context.Context, domain string, s *sources.Session, emit func(string)) error
}

func (f *fake) Info() sources.Info { return f.info }
func (f *fake) Enumerate(ctx context.Context, d string, s *sources.Session, emit func(string)) error {
	return f.fn(ctx, d, s, emit)
}

func src(name string, fn func(ctx context.Context, d string, s *sources.Session, emit func(string)) error) Task {
	return Task{Source: &fake{info: sources.Info{Name: name}, fn: fn}}
}

func emits(name string, raws ...string) Task {
	return src(name, func(_ context.Context, _ string, _ *sources.Session, emit func(string)) error {
		for _, r := range raws {
			emit(r)
		}
		return nil
	})
}

func failing(name string, err error) Task {
	return src(name, func(context.Context, string, *sources.Session, func(string)) error { return err })
}

func TestDedupeAndProvenance(t *testing.T) {
	tasks := []Task{
		emits("b", "API.example.com", "*.www.example.com", "example.com", "notexample.com", "x.evil.test"),
		emits("a", "api.example.com.", "https://mail.example.com/x", "api.example.com"),
	}
	res := Run(context.Background(), []string{"example.com"}, tasks, Options{})
	want := []Finding{
		{"example.com", "api.example.com", []string{"a", "b"}},
		{"example.com", "mail.example.com", []string{"a"}},
		{"example.com", "www.example.com", []string{"b"}},
	}
	if !reflect.DeepEqual(res.Findings, want) {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if res.Outcomes[0].Source != "a" || res.Outcomes[0].Count != 2 || res.Outcomes[1].Count != 2 {
		t.Fatalf("outcomes = %+v", res.Outcomes)
	}
	if s := res.Summary(); s.OK != 2 || s.Total != 3 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestMultiDomainOrderAndScope(t *testing.T) {
	tk := src("s", func(_ context.Context, d string, _ *sources.Session, emit func(string)) error {
		emit("z." + d)
		emit("a." + d)
		emit("a.other.test") // out of scope for both
		return nil
	})
	res := Run(context.Background(), []string{"zeta.org", "alpha.com"}, []Task{tk}, Options{})
	var got []string
	for _, f := range res.Findings {
		got = append(got, f.Subdomain)
	}
	want := []string{"a.zeta.org", "z.zeta.org", "a.alpha.com", "z.alpha.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestErrorClassification(t *testing.T) {
	cases := map[string]struct {
		err  error
		kind string
	}{
		"nokey": {fmt.Errorf("x: %w", sources.ErrNoKey), "no_key"},
		"auth":  {sources.ErrAuth, "auth"},
		"rate":  {sources.ErrRateLimited, "rate_limited"},
		"tmo":   {sources.ErrTimeout, "timeout"},
		"dl":    {context.DeadlineExceeded, "timeout"},
		"unexp": {sources.ErrUnexpected, "unexpected"},
		"other": {errors.New("boom"), "unexpected"},
		"unavl": {sources.ErrUnavailable, "unavailable"},
		"block": {sources.ErrBlockedHost, "blocked"},
	}
	var tasks []Task
	for n, c := range cases {
		tasks = append(tasks, failing(n, c.err))
	}
	tasks = append(tasks, emits("good", "a.example.com"), emits("zero"))
	res := Run(context.Background(), []string{"example.com"}, tasks, Options{})
	for _, o := range res.Outcomes {
		if c, ok := cases[o.Source]; ok {
			if o.Status != StatusFailed || o.Kind != c.kind {
				t.Errorf("%s: %+v want kind %s", o.Source, o, c.kind)
			}
		}
		if o.Source == "good" && (o.Status != StatusOK || o.Count != 1) {
			t.Errorf("good: %+v", o)
		}
		if o.Source == "zero" && (o.Status != StatusOK || o.Count != 0) {
			t.Errorf("zero results must be ok: %+v", o)
		}
	}
	if s := res.Summary(); s.OK != 2 || s.Failed != len(cases) {
		t.Fatalf("%+v", s)
	}
}

func TestDisabledErrorIsSkipped(t *testing.T) {
	res := Run(context.Background(), []string{"example.com"}, []Task{failing("d", sources.ErrDisabled)}, Options{})
	if res.Outcomes[0].Status != StatusSkipped {
		t.Fatalf("%+v", res.Outcomes[0])
	}
}

func TestPanicIsolated(t *testing.T) {
	tasks := []Task{
		src("boom", func(context.Context, string, *sources.Session, func(string)) error { panic("kaboom") }),
		emits("fine", "a.example.com"),
	}
	res := Run(context.Background(), []string{"example.com"}, tasks, Options{Concurrency: 1})
	if res.Outcomes[0].Kind != "unexpected" || !strings.Contains(res.Outcomes[0].Message, "kaboom") {
		t.Fatalf("%+v", res.Outcomes[0])
	}
	if res.Outcomes[1].Status != StatusOK || len(res.Findings) != 1 {
		t.Fatalf("%+v %+v", res.Outcomes[1], res.Findings)
	}
}

func TestSkipRules(t *testing.T) {
	need := &fake{info: sources.Info{Name: "needs", Auth: sources.AuthRequired, CredFields: []string{"api_id", "api_secret"}}, fn: func(context.Context, string, *sources.Session, func(string)) error {
		t.Error("must not run")
		return nil
	}}
	dis := &fake{info: sources.Info{Name: "off", Disabled: true, DisabledReason: "service shut down"}}
	opt := &fake{info: sources.Info{Name: "opt", Auth: sources.AuthOptional, CredFields: []string{"api_key"}}, fn: func(_ context.Context, _ string, s *sources.Session, _ func(string)) error {
		return nil
	}}
	res := Run(context.Background(), []string{"example.com"}, []Task{
		{Source: need, Creds: map[string]string{"api_id": "x"}}, {Source: dis}, {Source: opt},
	}, Options{})
	byName := map[string]Outcome{}
	for _, o := range res.Outcomes {
		byName[o.Source] = o
	}
	if o := byName["needs"]; o.Status != StatusSkipped || o.Message != "missing credentials: LUNATIC_NEEDS_API_SECRET" {
		t.Errorf("%+v", o)
	}
	if o := byName["off"]; o.Status != StatusSkipped || o.Message != "service shut down" {
		t.Errorf("%+v", o)
	}
	if o := byName["opt"]; o.Status != StatusOK {
		t.Errorf("%+v", o)
	}
}

func TestSecretsRedactedInOutcome(t *testing.T) {
	tk := failing("leaky", errors.New("bad response for key=TOPSECRET token=abc"))
	tk.Creds = map[string]string{"api_key": "TOPSECRET"}
	var lines []string
	tk2 := src("logger", func(_ context.Context, _ string, s *sources.Session, _ func(string)) error {
		s.Log("using %s", s.Creds["api_key"])
		return nil
	})
	tk2.Creds = map[string]string{"api_key": "SEKRIT"}
	res := Run(context.Background(), []string{"example.com"}, []Task{tk, tk2}, Options{
		Concurrency: 1,
		Verbose:     func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) },
	})
	if strings.Contains(res.Outcomes[0].Message, "TOPSECRET") || strings.Contains(res.Outcomes[0].Message, "abc") {
		t.Fatalf("leak: %q", res.Outcomes[0].Message)
	}
	if len(lines) != 1 || strings.Contains(lines[0], "SEKRIT") || !strings.Contains(lines[0], "logger") {
		t.Fatalf("verbose lines: %q", lines)
	}
}

func TestDeterminism(t *testing.T) {
	mk := func(rev bool) []Task {
		ts := []Task{}
		for i := 0; i < 8; i++ {
			i := i
			ts = append(ts, src(fmt.Sprintf("s%d", i), func(_ context.Context, _ string, _ *sources.Session, emit func(string)) error {
				for j := 0; j < 20; j++ {
					emit(fmt.Sprintf("h%d.example.com", (i*7+j*3)%25))
				}
				return nil
			}))
		}
		if rev {
			for l, r := 0, len(ts)-1; l < r; l, r = l+1, r-1 {
				ts[l], ts[r] = ts[r], ts[l]
			}
		}
		return ts
	}
	norm := func(r *Results) ([]Finding, []string) {
		var names []string
		for _, o := range r.Outcomes {
			names = append(names, fmt.Sprint(o.Source, o.Count))
		}
		return r.Findings, names
	}
	f1, o1 := norm(Run(context.Background(), []string{"example.com"}, mk(false), Options{Concurrency: 3}))
	f2, o2 := norm(Run(context.Background(), []string{"example.com"}, mk(true), Options{Concurrency: 8}))
	if !reflect.DeepEqual(f1, f2) || !reflect.DeepEqual(o1, o2) {
		t.Fatal("results differ between runs")
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 4)
	block := func(name string) Task {
		return src(name, func(ctx context.Context, _ string, _ *sources.Session, emit func(string)) error {
			emit("early.example.com")
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		})
	}
	go func() {
		<-started
		cancel()
	}()
	res := Run(ctx, []string{"example.com"}, []Task{block("a"), block("b"), emits("c", "x.example.com")}, Options{Concurrency: 1})
	if ctx.Err() == nil {
		t.Fatal("ctx not canceled")
	}
	canceled := 0
	for _, o := range res.Outcomes {
		if o.Kind == "canceled" {
			canceled++
		}
	}
	if canceled != 3 {
		t.Fatalf("outcomes: %+v", res.Outcomes)
	}
	if len(res.Findings) == 0 || res.Findings[0].Subdomain != "early.example.com" {
		t.Fatalf("partial results lost: %+v", res.Findings)
	}
}

func TestSourceTimeout(t *testing.T) {
	tk := src("slow", func(ctx context.Context, _ string, _ *sources.Session, _ func(string)) error {
		<-ctx.Done()
		return errors.New("gave up") // untyped error after deadline is still a timeout
	})
	start := time.Now()
	res := Run(context.Background(), []string{"example.com"}, []Task{tk}, Options{SourceTimeout: 50 * time.Millisecond})
	if res.Outcomes[0].Kind != "timeout" || time.Since(start) > 2*time.Second {
		t.Fatalf("%+v", res.Outcomes[0])
	}
}

func TestSessionDefaults(t *testing.T) {
	tk := src("chk", func(_ context.Context, d string, s *sources.Session, _ func(string)) error {
		if s.HTTP == nil || s.MaxPages != 10 {
			return errors.New("bad session")
		}
		// passive guard covers every input domain
		if _, err := s.HTTP.Get(context.Background(), "http://api.second.org/", nil); !errors.Is(err, sources.ErrBlockedHost) {
			return fmt.Errorf("guard: %v", err)
		}
		return nil
	})
	res := Run(context.Background(), []string{"example.com", "second.org"}, []Task{tk}, Options{})
	if res.Outcomes[0].Status != StatusOK {
		t.Fatalf("%+v", res.Outcomes[0])
	}
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestQueryParamAndOptionalSecretsRedacted(t *testing.T) {
	const key, opt, host = "QKEYSECRET1", "OPTSECRET22", "zoomeye.hk"
	info := sources.Info{Name: "q", Auth: sources.AuthRequired, CredFields: []string{"api_key"}, OptCredFields: []string{"email"}}
	task := Task{Creds: map[string]string{"api_key": host + ":" + "COMPOSITEKEY9", "email": opt}, Source: &fake{info: info,
		fn: func(ctx context.Context, d string, s *sources.Session, emit func(string)) error {
			err := s.HTTP.GetJSON(ctx, "https://provider.test/x?apikey=COMPOSITEKEY9&email="+opt+"&other="+key, nil, &struct{}{})
			return err
		}}}
	var logs []string
	var mu sync.Mutex
	res := Run(context.Background(), []string{"example.com"}, []Task{task}, Options{
		Verbose: func(f string, a ...any) { mu.Lock(); logs = append(logs, fmt.Sprintf(f, a...)); mu.Unlock() },
		Transport: rtFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 500, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("boom " + r.URL.RawQuery)), Request: r}, nil
		}),
	})
	all := strings.Join(logs, "\n") + res.Outcomes[0].Message
	for _, leak := range []string{"COMPOSITEKEY9", opt} {
		if strings.Contains(all, leak) {
			t.Fatalf("%q leaked: %s", leak, all)
		}
	}
	if res.Outcomes[0].Status != StatusFailed {
		t.Fatalf("outcome %+v", res.Outcomes[0])
	}
}

func TestSkipReasonIgnoresOptionalFields(t *testing.T) {
	info := sources.Info{Name: "fofa", Auth: sources.AuthRequired, CredFields: []string{"key"}, OptCredFields: []string{"email"}}
	if r := SkipReason(info, map[string]string{"key": "k"}); r != "" {
		t.Fatalf("optional field must not be required: %q", r)
	}
	if r := SkipReason(info, map[string]string{"email": "e"}); !strings.Contains(r, "LUNATIC_FOFA_KEY") || strings.Contains(r, "EMAIL") {
		t.Fatalf("reason %q", r)
	}
}

func TestTwoPhaseIPs(t *testing.T) {
	var order []string
	var mu sync.Mutex
	mark := func(s string) { mu.Lock(); order = append(order, s); mu.Unlock() }
	p1 := func(name string, ips ...string) Task {
		return src(name, func(_ context.Context, d string, s *sources.Session, emit func(string)) error {
			time.Sleep(20 * time.Millisecond)
			for _, ip := range ips {
				s.IP(ip)
			}
			emit("a." + d)
			mark("p1-" + name)
			return nil
		})
	}
	var gotIPs []string
	p2 := Task{Source: &fake{info: sources.Info{Name: "second", Phase2: true}, fn: func(_ context.Context, d string, s *sources.Session, emit func(string)) error {
		mark("p2")
		gotIPs = append([]string(nil), s.IPs...)
		if s.ReportIP != nil {
			t.Error("phase-2 session must not have ReportIP")
		}
		emit("found." + d)
		emit("out.other.test")
		return nil
	}}}
	tasks := []Task{
		p2,
		p1("x", "93.184.216.34", "10.0.0.1", "127.0.0.1", "192.168.1.1", "169.254.1.1", "203.0.113.5", "::1", "fe80::1", "garbage", "93.184.216.34", " 2606:2800:220:1::1 "),
		p1("y", "8.8.8.8", "::ffff:1.1.1.1"),
	}
	res := Run(context.Background(), []string{"example.com"}, tasks, Options{})
	if len(order) != 3 || order[2] != "p2" {
		t.Fatalf("order %v", order)
	}
	want := []string{"1.1.1.1", "2606:2800:220:1::1", "8.8.8.8", "93.184.216.34"}
	if !reflect.DeepEqual(gotIPs, want) {
		t.Fatalf("ips %v want %v", gotIPs, want)
	}
	seen := map[string][]string{}
	for _, f := range res.Findings {
		seen[f.Subdomain] = f.Sources
	}
	if !reflect.DeepEqual(seen["found.example.com"], []string{"second"}) || seen["out.other.test"] != nil {
		t.Fatalf("findings %+v", res.Findings)
	}
}

func TestPhase2NoIPsAndPerDomain(t *testing.T) {
	var got [][]string
	p2 := Task{Source: &fake{info: sources.Info{Name: "second", Phase2: true}, fn: func(_ context.Context, d string, s *sources.Session, emit func(string)) error {
		got = append(got, append([]string{d}, s.IPs...))
		return nil
	}}}
	p1 := src("one", func(_ context.Context, d string, s *sources.Session, emit func(string)) error {
		if d == "b.test" {
			s.IP("8.8.4.4")
		}
		return nil
	})
	res := Run(context.Background(), []string{"a.test", "b.test"}, []Task{p1, p2}, Options{})
	want := [][]string{{"a.test"}, {"b.test", "8.8.4.4"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	for _, o := range res.Outcomes {
		if o.Status != StatusOK {
			t.Fatalf("%+v", o)
		}
	}
}

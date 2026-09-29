// Package cli implements the lunatic command line.
//
// Exit codes:
//
//	0    every attempted source succeeded (zero results still counts as success)
//	1    usage, input or config error (nothing was run)
//	2    no source succeeded (all attempted sources failed, or none were usable)
//	3    partial success: at least one source succeeded and at least one failed
//	130  interrupted (SIGINT/SIGTERM); partial results are still written
//
// Streams: results go to stdout only (data, never ANSI). The banner, warnings,
// verbose logs and the summary go to stderr. --silent suppresses the banner,
// warnings, verbose logs and the summary; only fatal errors reach stderr.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/lunalully/lunatic/internal/banner"
	"github.com/lunalully/lunatic/internal/config"
	"github.com/lunalully/lunatic/internal/output"
	"github.com/lunalully/lunatic/internal/runner"
	"github.com/lunalully/lunatic/internal/scope"
	"github.com/lunalully/lunatic/internal/sources"
	"github.com/lunalully/lunatic/internal/term"
	"github.com/lunalully/lunatic/internal/version"
)

// Exit codes (see package comment).
const (
	ExitOK          = 0
	ExitUsage       = 1
	ExitNoSuccess   = 2
	ExitPartial     = 3
	ExitInterrupted = 130
)

// env carries everything that tests need to inject.
type env struct {
	sources   []sources.Source
	lookupEnv func(string) (string, bool)
	stderrTTY bool
}

// testTransport, when non-nil, replaces the network transport of every source
// (test-only hook; there is deliberately no user-facing flag).
var testTransport http.RoundTripper

// Main runs the CLI and returns the process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, args, stdout, stderr, env{
		sources:   sources.All(),
		lookupEnv: os.LookupEnv,
		stderrTTY: term.IsTerminal(stderr),
	})
}

// listFlag collects comma-separated values; it may be given several times.
type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*l = append(*l, p)
		}
	}
	return nil
}

type options struct {
	domains, srcs, exclude                                   listFlag
	dList, outFile, cfgPath                                  string
	all, json, silent, list, verbose, noColor, version, help bool
	timeout, maxTime, concurrency                            int
}

const helpText = `lunatic - passive subdomain recon

Usage:
  lunatic -d example.com [flags]
  lunatic -dL domains.txt [flags]

Input:
  -d domain               target domain (repeatable or comma-separated)
  -dL file                file with one domain per line ('#' comments and blank lines ignored)

Sources:
  -s list                 run only these sources (comma-separated names)
  -es, --exclude-sources list
                          skip these sources (comma-separated names)
  --all                   use every non-disabled source (sources missing keys are reported as skipped)
  --list-sources          print the source table and exit

Output:
  -o file                 also write results to file (created/truncated); results always go to stdout
  --json, -oJ             JSON Lines output: {"domain","subdomain","sources"}
  --silent                only results on stdout; no banner, warnings, verbose logs or summary

Runtime:
  --config path           YAML config with API keys (default: $XDG_CONFIG_HOME/lunatic/config.yaml
                          or ~/.config/lunatic/config.yaml); env LUNATIC_<SOURCE>_<FIELD> overrides it
  --timeout seconds       per-source timeout for each domain (default 90)
  --max-time minutes      overall time limit, 0 = none (default 10)
  --concurrency n         sources running at once (default 10)
  -v, --verbose           verbose logs to stderr (secrets redacted)
  --no-color              disable colors (also: NO_COLOR env, TERM=dumb, non-TTY stderr)
  --version               print version and exit
  -h, --help              show this help

Exit codes:
  0 all attempted sources succeeded   1 usage/input/config error
  2 no source succeeded               3 partial success (some sources failed)
  130 interrupted
`

func newFlagSet(o *options) *flag.FlagSet {
	fs := flag.NewFlagSet("lunatic", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(&o.domains, "d", "")
	fs.StringVar(&o.dList, "dL", "", "")
	fs.StringVar(&o.outFile, "o", "", "")
	fs.Var(&o.srcs, "s", "")
	fs.Var(&o.exclude, "exclude-sources", "")
	fs.Var(&o.exclude, "es", "")
	fs.BoolVar(&o.all, "all", false, "")
	fs.BoolVar(&o.json, "json", false, "")
	fs.BoolVar(&o.json, "oJ", false, "")
	fs.BoolVar(&o.silent, "silent", false, "")
	fs.BoolVar(&o.list, "list-sources", false, "")
	fs.StringVar(&o.cfgPath, "config", "", "")
	fs.IntVar(&o.timeout, "timeout", 90, "")
	fs.IntVar(&o.maxTime, "max-time", 10, "")
	fs.IntVar(&o.concurrency, "concurrency", 10, "")
	fs.BoolVar(&o.verbose, "verbose", false, "")
	fs.BoolVar(&o.verbose, "v", false, "")
	fs.BoolVar(&o.noColor, "no-color", false, "")
	fs.BoolVar(&o.version, "version", false, "")
	fs.BoolVar(&o.help, "help", false, "")
	fs.BoolVar(&o.help, "h", false, "")
	return fs
}

// lockedWriter serializes concurrent verbose logging.
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, e env) int {
	var o options
	fs := newFlagSet(&o)
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "lunatic: "+format+"\n", a...)
		return ExitUsage
	}
	if err := fs.Parse(args); err != nil {
		return fail("%v (see lunatic --help)", err)
	}
	if o.help {
		io.WriteString(stdout, helpText)
		return ExitOK
	}
	if o.version {
		fmt.Fprintf(stdout, "lunatic %s\n", version.Version)
		return ExitOK
	}
	if fs.NArg() > 0 {
		return fail("unexpected argument %q (use -d example.com)", fs.Arg(0))
	}
	if o.timeout < 1 || o.concurrency < 1 || o.maxTime < 0 {
		return fail("--timeout and --concurrency must be >= 1 and --max-time >= 0")
	}
	if o.all && len(o.srcs) > 0 {
		return fail("--all and -s cannot be combined")
	}

	getenv := func(k string) string { v, _ := e.lookupEnv(k); return v }
	cfg, warns, err := config.Load(o.cfgPath, getenv)
	if err != nil {
		return fail("%v", err)
	}
	lvl := term.ColorLevel(e.lookupEnv, e.stderrTTY, o.noColor)
	if !o.silent {
		for _, w := range warns {
			fmt.Fprintln(stderr, term.Paint(lvl, "yellow", "warning: "+w))
		}
	}

	byName := map[string]sources.Source{}
	var names []string
	for _, s := range e.sources {
		byName[s.Info().Name] = s
		names = append(names, s.Info().Name)
	}
	sort.Strings(names)
	creds := func(s sources.Source) map[string]string {
		return cfg.Creds(s.Info().Name, s.Info().AllCredFields())
	}

	if o.list {
		listSources(stdout, e.sources, creds)
		return ExitOK
	}

	// Domains.
	raw := append([]string(nil), o.domains...)
	var problems []string
	if o.dList != "" {
		data, err := os.ReadFile(o.dList)
		if err != nil {
			return fail("cannot read -dL file: %v", err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if j := strings.Index(line, "#"); j >= 0 {
				line = line[:j]
			}
			if line = strings.TrimSpace(line); line == "" {
				continue
			}
			if _, err := scope.ParseDomain(line); err != nil {
				problems = append(problems, fmt.Sprintf("%s:%d: %v", o.dList, i+1, err))
				continue
			}
			raw = append(raw, line)
		}
	}
	var domains []string
	seen := map[string]bool{}
	for _, r := range raw {
		d, err := scope.ParseDomain(r)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if !seen[d] {
			seen[d] = true
			domains = append(domains, d)
		}
	}
	if len(problems) > 0 {
		return fail("invalid input:\n  %s", strings.Join(problems, "\n  "))
	}
	if len(domains) == 0 {
		return fail("no domain given; use -d example.com or -dL file (see lunatic --help)")
	}

	// Source selection.
	checkNames := func(flagName string, list []string) ([]string, bool) {
		var out []string
		var unknown []string
		for _, n := range list {
			n = strings.ToLower(n)
			if _, ok := byName[n]; !ok {
				unknown = append(unknown, n)
				continue
			}
			out = append(out, n)
		}
		if len(unknown) > 0 {
			fail("unknown source(s) in %s: %s\nvalid sources: %s", flagName, strings.Join(unknown, ", "), strings.Join(names, ", "))
			return nil, false
		}
		return out, true
	}
	chosen, ok := checkNames("-s", o.srcs)
	if !ok {
		return ExitUsage
	}
	excluded, ok := checkNames("--exclude-sources", o.exclude)
	if !ok {
		return ExitUsage
	}
	skipSet := map[string]bool{}
	for _, n := range excluded {
		skipSet[n] = true
	}
	var tasks []runner.Task
	add := func(s sources.Source) {
		if !skipSet[s.Info().Name] {
			tasks = append(tasks, runner.Task{Source: s, Creds: creds(s)})
		}
	}
	switch {
	case len(chosen) > 0:
		done := map[string]bool{}
		for _, n := range chosen {
			if !done[n] {
				done[n] = true
				add(byName[n])
			}
		}
	case o.all:
		for _, s := range e.sources {
			if !s.Info().Disabled {
				add(s)
			}
		}
	default:
		for _, s := range e.sources {
			i := s.Info()
			if i.Default && !i.Disabled && runner.SkipReason(i, creds(s)) == "" {
				add(s)
			}
		}
	}

	var outF *os.File
	if o.outFile != "" {
		outF, err = os.OpenFile(o.outFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			return fail("cannot open output file: %v", err)
		}
		defer outF.Close()
	}

	if !o.silent && e.stderrTTY {
		banner.Print(stderr, version.Version, lvl)
	}

	runCtx := ctx
	if o.maxTime > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, time.Duration(o.maxTime)*time.Minute)
		defer cancel()
	}
	opts := runner.Options{
		Concurrency:   o.concurrency,
		SourceTimeout: time.Duration(o.timeout) * time.Second,
		Transport:     testTransport,
	}
	if o.verbose && !o.silent {
		lw := &lockedWriter{w: stderr}
		opts.Verbose = func(format string, a ...any) {
			fmt.Fprintf(lw, "[verbose] "+format+"\n", a...)
		}
	}
	res := runner.Run(runCtx, domains, tasks, opts)

	write := output.WriteTXT
	if o.json {
		write = output.WriteJSONL
	}
	code := ExitOK
	if err := write(stdout, res.Findings); err != nil {
		fmt.Fprintf(stderr, "lunatic: writing results: %v\n", err)
		code = ExitUsage
	}
	if outF != nil {
		if err := write(outF, res.Findings); err != nil {
			fmt.Fprintf(stderr, "lunatic: writing %s: %v\n", o.outFile, err)
			code = ExitUsage
		}
	}
	if !o.silent {
		printSummary(stderr, res, len(domains), lvl)
	}

	if ctx.Err() != nil {
		return ExitInterrupted
	}
	if code != ExitOK {
		return code
	}
	s := res.Summary()
	switch {
	case s.OK == 0:
		if o.silent {
			fmt.Fprintln(stderr, "lunatic: no source succeeded")
		}
		return ExitNoSuccess
	case s.Failed > 0:
		return ExitPartial
	}
	return ExitOK
}

func printSummary(w io.Writer, res *runner.Results, ndomains int, lvl term.Level) {
	width := 0
	for _, o := range res.Outcomes {
		if len(o.Source) > width {
			width = len(o.Source)
		}
	}
	fmt.Fprintln(w)
	if len(res.Outcomes) == 0 {
		fmt.Fprintln(w, "  no usable sources selected (see --list-sources)")
	}
	for _, o := range res.Outcomes {
		pad := strings.Repeat(" ", width-len(o.Source))
		switch o.Status {
		case runner.StatusOK:
			fmt.Fprintf(w, "  %s %s%s  ok %d\n", term.Paint(lvl, "green", "+"), o.Source, pad, o.Count)
		case runner.StatusSkipped:
			fmt.Fprintf(w, "  %s %s%s  skipped (%s)\n", term.Paint(lvl, "yellow", "-"), o.Source, pad, o.Message)
		default:
			fmt.Fprintf(w, "  %s %s%s  failed (%s): %s\n", term.Paint(lvl, "red", "!"), o.Source, pad, o.Kind, o.Message)
		}
	}
	s := res.Summary()
	fmt.Fprintf(w, "\n  %d unique subdomain(s) across %d domain(s); sources: %d ok, %d skipped, %d failed\n",
		s.Total, ndomains, s.OK, s.Skipped, s.Failed)
}

func listSources(w io.Writer, srcs []sources.Source, creds func(sources.Source) map[string]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tAUTH\tSTATUS\tDEFAULT\tNOTE")
	sorted := append([]sources.Source(nil), srcs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Info().Name < sorted[j].Info().Name })
	for _, s := range sorted {
		i := s.Info()
		status, note := "ready", ""
		var envs, optEnvs []string
		for _, f := range i.CredFields {
			envs = append(envs, config.EnvName(i.Name, f))
		}
		for _, f := range i.OptCredFields {
			optEnvs = append(optEnvs, config.EnvName(i.Name, f))
		}
		switch {
		case i.Disabled:
			status, note = "disabled", i.DisabledReason
		case i.Auth == sources.AuthRequired:
			note = "env: " + strings.Join(envs, ", ")
			if runner.SkipReason(i, creds(s)) != "" {
				status = "needs key"
			}
		case i.Auth == sources.AuthOptional && len(envs) > 0:
			note = "optional env: " + strings.Join(envs, ", ")
		}
		if len(optEnvs) > 0 && !i.Disabled {
			if note != "" {
				note += "; "
			}
			note += "optional env: " + strings.Join(optEnvs, ", ")
		}
		def := "no"
		if i.Default {
			def = "yes"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", i.Name, i.Auth, status, def, note)
	}
	tw.Flush()
}

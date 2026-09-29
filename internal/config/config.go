// Package config loads per-source credentials from a YAML file and the
// environment. Precedence: env var LUNATIC_<SOURCE>_<FIELD> > config file.
// Empty or placeholder values count as absent. Values are never printed.
//
// The YAML reader is a deliberately tiny subset parser (block mappings, "#"
// comments, plain/single/double-quoted scalars) sufficient for the documented
// format, so the project needs no YAML dependency.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds credentials read from a file.
type Config struct {
	Path    string                       // file actually read ("" if none)
	Sources map[string]map[string]string // source -> field -> value
	getenv  func(string) string
}

// DefaultPath returns $XDG_CONFIG_HOME/lunatic/config.yaml, falling back to
// ~/.config/lunatic/config.yaml ("" if no home directory is known).
func DefaultPath(getenv func(string) string) string {
	if x := getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "lunatic", "config.yaml")
	}
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return filepath.Join(h, ".config", "lunatic", "config.yaml")
	}
	return ""
}

// Load reads the config. If path is empty the default path is used and a
// missing file is fine; an explicit path that cannot be read is an error.
// warnings are human-readable notices (e.g. world-readable file).
func Load(path string, getenv func(string) string) (cfg *Config, warnings []string, err error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg = &Config{Sources: map[string]map[string]string{}, getenv: getenv}
	explicit := path != ""
	if !explicit {
		path = DefaultPath(getenv)
		if path == "" {
			return cfg, nil, nil
		}
	}
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		if !explicit && errors.Is(rerr, os.ErrNotExist) {
			return cfg, nil, nil
		}
		return nil, nil, fmt.Errorf("config file: %w", rerr)
	}
	if fi, serr := os.Stat(path); serr == nil && fi.Mode().Perm()&0o004 != 0 {
		warnings = append(warnings, fmt.Sprintf("config file %s is world-readable; run: chmod 600 %s", path, path))
	}
	srcs, perr := parse(string(data))
	if perr != nil {
		return nil, nil, fmt.Errorf("config file %s: %w", path, perr)
	}
	cfg.Path = path
	cfg.Sources = srcs
	return cfg, warnings, nil
}

// EnvName returns the environment variable for a credential field, e.g.
// EnvName("shodan","api_key") == "LUNATIC_SHODAN_API_KEY".
func EnvName(source, field string) string {
	f := func(s string) string {
		return strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r - 32
			case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				return r
			}
			return '_'
		}, s)
	}
	return "LUNATIC_" + f(source) + "_" + f(field)
}

// IsPlaceholder reports whether v should count as "no value".
func IsPlaceholder(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "<") {
		return true
	}
	u := strings.ToUpper(v)
	switch u {
	case "CHANGEME", "CHANGE_ME", "TODO", "NONE", "NULL":
		return true
	}
	return strings.HasPrefix(u, "YOUR_") || strings.HasPrefix(u, "YOUR-")
}

// Creds returns the usable credentials for a source (only the listed fields;
// env beats file; placeholders dropped).
func (c *Config) Creds(source string, fields []string) map[string]string {
	out := map[string]string{}
	for _, f := range fields {
		v := c.getenv(EnvName(source, f))
		if IsPlaceholder(v) {
			v = c.Sources[source][f]
		}
		if !IsPlaceholder(v) {
			out[f] = strings.TrimSpace(v)
		}
	}
	return out
}

// parse reads the subset of YAML described in the package comment and
// returns the "sources" mapping. Other top-level keys are ignored.
func parse(text string) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	type frame struct {
		indent int
		key    string
	}
	var stack []frame
	for i, line := range strings.Split(text, "\n") {
		n := i + 1
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == "---" || strings.TrimSpace(line) == "..." {
			continue
		}
		body := strings.TrimLeft(line, " ")
		if strings.HasPrefix(body, "\t") {
			return nil, fmt.Errorf("line %d: tabs are not allowed for indentation", n)
		}
		indent := len(line) - len(body)
		key, val, err := splitKV(body)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", n, err)
		}
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		path := make([]string, 0, len(stack)+1)
		for _, f := range stack {
			path = append(path, f.key)
		}
		if val == "" && !quoted(body) {
			// mapping header (value empty and not an explicit empty string)
			stack = append(stack, frame{indent, key})
			if len(path) == 1 && path[0] == "sources" {
				if out[key] == nil {
					out[key] = map[string]string{}
				}
			}
			continue
		}
		if len(path) == 2 && path[0] == "sources" {
			if out[path[1]] == nil {
				out[path[1]] = map[string]string{}
			}
			out[path[1]][key] = val
		} else if len(path) == 0 && key == "sources" && (val == "{}" || val == "null" || val == "~") {
			continue
		}
	}
	return out, nil
}

// quoted reports whether the value part of "key: value" is a quoted scalar.
func quoted(body string) bool {
	i := strings.Index(body, ":")
	if i < 0 {
		return false
	}
	v := strings.TrimSpace(body[i+1:])
	return strings.HasPrefix(v, `"`) || strings.HasPrefix(v, `'`)
}

func stripComment(s string) string {
	var q rune
	for i, r := range s {
		switch {
		case q != 0:
			if r == q {
				q = 0
			}
		case r == '"' || r == '\'':
			q = r
		case r == '#' && (i == 0 || s[i-1] == ' '):
			return s[:i]
		}
	}
	return s
}

func splitKV(body string) (key, val string, err error) {
	body = strings.TrimSpace(stripComment(body))
	i := strings.Index(body, ":")
	if i <= 0 {
		return "", "", errors.New("expected 'key: value'")
	}
	key = strings.Trim(strings.TrimSpace(body[:i]), `"'`)
	rest := strings.TrimSpace(body[i+1:])
	switch {
	case strings.HasPrefix(rest, `"`):
		u, uerr := strconv.Unquote(rest)
		if uerr != nil {
			return "", "", fmt.Errorf("bad double-quoted value for %q", key)
		}
		return key, u, nil
	case strings.HasPrefix(rest, `'`):
		if len(rest) < 2 || !strings.HasSuffix(rest, `'`) {
			return "", "", fmt.Errorf("bad single-quoted value for %q", key)
		}
		return key, strings.ReplaceAll(rest[1:len(rest)-1], "''", "'"), nil
	case rest == "~" || rest == "null":
		return key, "", nil
	case strings.HasPrefix(rest, "-") && rest != "-":
		return key, rest, nil
	}
	return key, rest, nil
}

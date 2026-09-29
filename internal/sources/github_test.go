package sources

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestGithubSuccessExtractsOnlyHostnames(t *testing.T) {
	var auth, accept, q string
	got, err := g2run(t, github{}, &githubBaseURL, func(w http.ResponseWriter, r *http.Request) {
		auth, accept, q = r.Header.Get("Authorization"), r.Header.Get("Accept"), r.URL.Query().Get("q")
		w.Write(g2fixture(t, "github", "page1.json"))
	}, map[string]string{"token": "PAT"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "api.example.com", "docs.example.com")
	for _, g := range got {
		if strings.Contains(g, "SECRET") {
			t.Fatal("leaked fragment content")
		}
	}
	if auth != "Bearer PAT" || !strings.Contains(accept, "text-match") || q != `"example.com"` {
		t.Fatalf("auth=%q accept=%q q=%q", auth, accept, q)
	}
}

func TestGithubPaginationMaxPages(t *testing.T) {
	n := 0
	_, err := g2run(t, github{}, &githubBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		var sb strings.Builder
		sb.WriteString(`{"total_count":900,"items":[`)
		for i := 0; i < githubPerPage; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`{"text_matches":[{"fragment":"h` + strconv.Itoa(n) + "x" + strconv.Itoa(i) + `.example.com"}]}`)
		}
		sb.WriteString(`]}`)
		if r.URL.Query().Get("page") != strconv.Itoa(n) {
			t.Errorf("page param %q at request %d", r.URL.Query().Get("page"), n)
		}
		w.Write([]byte(sb.String()))
	}, map[string]string{"token": "PAT"}, 3)
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestGithubEmptyAnd422OnLaterPage(t *testing.T) {
	c := map[string]string{"token": "PAT"}
	got, err := g2run(t, github{}, &githubBaseURL, g2serve(200, g2fixture(t, "github", "empty.json")), c, 10)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
	_, err = g2run(t, github{}, &githubBaseURL, g2serve(422, []byte(`{"message":"Validation Failed"}`)), c, 10)
	g2is(t, err, ErrUnexpected)
}

func TestGithubRateLimitOn403(t *testing.T) {
	c := map[string]string{"token": "PAT"}
	_, err := g2run(t, github{}, &githubBaseURL, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(403)
		w.Write(g2fixture(t, "github", "ratelimit.json"))
	}, c, 10)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, github{}, &githubBaseURL, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(403)
	}, c, 10)
	g2is(t, err, ErrRateLimited)
	// plain 403 without rate-limit signals is an auth/permission failure
	_, err = g2run(t, github{}, &githubBaseURL, g2serve(403, []byte(`{"message":"Forbidden"}`)), c, 10)
	g2is(t, err, ErrAuth)
}

func TestGithubErrors(t *testing.T) {
	g2errors(t, github{}, &githubBaseURL, map[string]string{"token": "PAT"}, "<html>")
}

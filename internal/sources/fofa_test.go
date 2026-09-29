package sources

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestFofaSuccess(t *testing.T) {
	var key, q, fields string
	got, err := g2run(t, fofa{}, &fofaBaseURL, func(w http.ResponseWriter, r *http.Request) {
		key, fields = r.URL.Query().Get("key"), r.URL.Query().Get("fields")
		b, _ := base64.StdEncoding.DecodeString(r.URL.Query().Get("qbase64"))
		q = string(b)
		w.Write(g2fixture(t, "fofa", "success.json"))
	}, map[string]string{"key": "K"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "mail.example.com")
	if key != "K" || q != `domain="example.com"` || fields != "host" {
		t.Fatalf("key=%q q=%q fields=%q", key, q, fields)
	}
}

func TestFofaPaginationMaxPages(t *testing.T) {
	n := 0
	_, err := g2run(t, fofa{}, &fofaBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		var sb strings.Builder
		sb.WriteString(`{"error":false,"results":[`)
		for i := 0; i < fofaSize; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(`"h` + strconv.Itoa(n) + "x" + strconv.Itoa(i) + `.example.com"`)
		}
		sb.WriteString(`]}`)
		w.Write([]byte(sb.String()))
	}, map[string]string{"key": "K"}, 3)
	if err != nil || n != 3 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestFofaEmpty(t *testing.T) {
	got, err := g2run(t, fofa{}, &fofaBaseURL, g2serve(200, []byte(`{"error":false,"errmsg":"","size":0,"results":[]}`)), map[string]string{"key": "K"}, 3)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestFofaErrorBodies(t *testing.T) {
	c := map[string]string{"key": "K"}
	_, err := g2run(t, fofa{}, &fofaBaseURL, g2serve(200, g2fixture(t, "fofa", "quota.json")), c, 3)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, fofa{}, &fofaBaseURL, g2serve(200, g2fixture(t, "fofa", "auth.json")), c, 3)
	g2is(t, err, ErrAuth)
	_, err = g2run(t, fofa{}, &fofaBaseURL, g2serve(200, []byte(`{"error":true,"errmsg":"weird"}`)), c, 3)
	g2is(t, err, ErrUnexpected)
}

func TestFofaErrors(t *testing.T) {
	g2errors(t, fofa{}, &fofaBaseURL, map[string]string{"key": "K"}, "<html>")
}

func TestFofaHost(t *testing.T) {
	for in, want := range map[string]string{"https://a.example.com:8443/x": "a.example.com", "b.example.com:25": "b.example.com", "c.example.com": "c.example.com"} {
		if got := fofaHost(in); got != want {
			t.Errorf("%q -> %q", in, got)
		}
	}
}

func TestFofaOptionalEmail(t *testing.T) {
	i := fofa{}.Info()
	if len(i.CredFields) != 1 || i.CredFields[0] != "key" || len(i.OptCredFields) != 1 || i.OptCredFields[0] != "email" {
		t.Fatalf("info %+v", i)
	}
	var withEmail, without []string
	body := g2fixture(t, "fofa", "success.json")
	if _, err := g2run(t, fofa{}, &fofaBaseURL, func(w http.ResponseWriter, r *http.Request) {
		withEmail = append(withEmail, r.URL.Query().Get("email"))
		w.Write(body)
	}, map[string]string{"key": "K", "email": "me@example.org"}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := g2run(t, fofa{}, &fofaBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if _, ok := r.URL.Query()["email"]; ok {
			without = append(without, "present")
		}
		w.Write(body)
	}, map[string]string{"key": "K"}, 1); err != nil {
		t.Fatal(err)
	}
	if len(withEmail) != 1 || withEmail[0] != "me@example.org" || len(without) != 0 {
		t.Fatalf("withEmail=%v without=%v", withEmail, without)
	}
}

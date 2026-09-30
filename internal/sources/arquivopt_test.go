package sources

import (
	"net/http"
	"testing"
)

func TestArquivoptSuccess(t *testing.T) {
	var q string
	got, err := g2run(t, arquivopt{}, &arquivoptBaseURL, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Path + "?" + r.URL.RawQuery
		w.Write(g2fixture(t, "arquivopt", "success.jsonl"))
	}, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "blog.example.com", "old.example.com")
	want := "/wayback/cdx?url=%2A.example.com&output=json&fields=url&limit=100000"
	if q != want {
		t.Fatalf("query %q want %q", q, want)
	}
}

func TestArquivoptSingleRequest(t *testing.T) {
	n := 0
	_, err := g2run(t, arquivopt{}, &arquivoptBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		w.Write([]byte(`{"url":"http://a.example.com/"}` + "\n"))
	}, nil, 10)
	if err != nil || n != 1 {
		t.Fatalf("requests=%d err=%v", n, err)
	}
}

func TestArquivoptEmpty(t *testing.T) {
	for _, body := range []string{"", "\n", "   \n"} {
		got, err := g2run(t, arquivopt{}, &arquivoptBaseURL, g2serve(200, []byte(body)), nil, 3)
		if err != nil || len(got) != 0 {
			t.Fatalf("body %q: got=%v err=%v", body, got, err)
		}
	}
}

func TestArquivoptErrors(t *testing.T) {
	_, err := g2run(t, arquivopt{}, &arquivoptBaseURL, g2serve(429, []byte("slow")), nil, 3)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, arquivopt{}, &arquivoptBaseURL, g2serve(503, []byte("x")), nil, 3)
	g2is(t, err, ErrUnavailable)
	_, err = g2run(t, arquivopt{}, &arquivoptBaseURL, g2serve(200, []byte("<html>oops</html>")), nil, 3)
	g2is(t, err, ErrUnexpected)
}

func TestArquivoptInfo(t *testing.T) {
	i := arquivopt{}.Info()
	if i.Default || i.Auth != AuthNone || i.RPS != 0.5 {
		t.Fatalf("%+v", i)
	}
}

package sources

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

func rapiddnsHTML(w http.ResponseWriter, b []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func TestRapiddnsInfo(t *testing.T) {
	i := rapiddns{}.Info()
	if i.Auth != AuthNone || i.Default || i.Disabled {
		t.Fatalf("%+v", i)
	}
}

func TestRapiddnsPagination(t *testing.T) {
	var pages []string
	merklemapServe(t, &rapiddnsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/subdomain/example.com" {
			t.Errorf("path %s", r.URL.Path)
		}
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		switch p {
		case "1":
			rapiddnsHTML(w, merklemapFixture(t, "rapiddns", "page1.html"))
		case "2":
			rapiddnsHTML(w, merklemapFixture(t, "rapiddns", "page2.html")) // one new name (mail)
		default:
			rapiddnsHTML(w, merklemapFixture(t, "rapiddns", "page2.html")) // no new names -> stop
		}
	})
	got, err := merklemapRun(rapiddns{}, merklemapSession("rapiddns", nil, 10))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com", "mail.example.com")
	if !reflect.DeepEqual(pages, []string{"1", "2", "3"}) {
		t.Fatalf("pages %v", pages)
	}
}

func TestRapiddnsMaxPages(t *testing.T) {
	n := 0
	hits := merklemapServe(t, &rapiddnsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		n++
		rapiddnsHTML(w, []byte("<td>h"+string(rune('a'+n))+".example.com</td>"))
	})
	if _, err := merklemapRun(rapiddns{}, merklemapSession("rapiddns", nil, 4)); err != nil || *hits != 4 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestRapiddnsEmpty(t *testing.T) {
	merklemapServe(t, &rapiddnsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		rapiddnsHTML(w, merklemapFixture(t, "rapiddns", "empty.html"))
	})
	got, err := merklemapRun(rapiddns{}, merklemapSession("rapiddns", nil, 5))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestRapiddnsCaptcha(t *testing.T) {
	hits := merklemapServe(t, &rapiddnsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		rapiddnsHTML(w, merklemapFixture(t, "rapiddns", "captcha.html"))
	})
	_, err := merklemapRun(rapiddns{}, merklemapSession("rapiddns", nil, 5))
	if !errors.Is(err, ErrUnavailable) || *hits != 1 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestRapiddnsStatuses(t *testing.T) {
	for _, c := range []struct {
		status int
		want   error
	}{{403, ErrUnavailable}, {429, ErrRateLimited}, {503, ErrUnavailable}} {
		c := c
		t.Run(http.StatusText(c.status), func(t *testing.T) {
			merklemapServe(t, &rapiddnsBaseURL, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(c.status) })
			_, err := merklemapRun(rapiddns{}, merklemapSession("rapiddns", nil, 5))
			if !errors.Is(err, c.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	t.Run("404", func(t *testing.T) {
		merklemapServe(t, &rapiddnsBaseURL, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
		if _, err := merklemapRun(rapiddns{}, merklemapSession("rapiddns", nil, 5)); err != nil {
			t.Fatal(err)
		}
	})
}

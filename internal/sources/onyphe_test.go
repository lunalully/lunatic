package sources

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

var onypheCreds = map[string]string{"api_key": "secret-key"}

func TestOnypheSuccessPagination(t *testing.T) {
	var pages []string
	merklemapServe(t, &onypheBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, onypheAllowedPath) {
			t.Errorf("forbidden path %s", r.URL.Path)
			http.Error(w, "no", 400)
			return
		}
		if r.Header.Get("Authorization") != "bearer secret-key" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		if q := r.URL.Query().Get("q"); q != "category:resolver domain:example.com" {
			t.Errorf("q %q", q)
		}
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		merklemapWrite(w, merklemapFixture(t, "onyphe", "page"+p+".json"))
	})
	got, err := merklemapRun(onyphe{}, merklemapSession("onyphe", onypheCreds, 10))
	merklemapExpect(t, got, err, "www.example.com", "mail.example.com", "smtp.example.com", "mx.example.com", "api.example.com")
	if !reflect.DeepEqual(pages, []string{"1", "2"}) {
		t.Fatalf("pages %v", pages)
	}
}

func TestOnypheMaxPages(t *testing.T) {
	hits := merklemapServe(t, &onypheBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"error":0,"max_page":50,"results":[{"hostname":"a.example.com"}]}`))
	})
	if _, err := merklemapRun(onyphe{}, merklemapSession("onyphe", onypheCreds, 3)); err != nil || *hits != 3 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestOnypheEmpty(t *testing.T) {
	merklemapServe(t, &onypheBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "onyphe", "empty.json"))
	})
	got, err := merklemapRun(onyphe{}, merklemapSession("onyphe", onypheCreds, 3))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestOnypheErrorIn200(t *testing.T) {
	merklemapServe(t, &onypheBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "onyphe", "error200.json"))
	})
	_, err := merklemapRun(onyphe{}, merklemapSession("onyphe", onypheCreds, 3))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

// Only the v2 search path may ever be requested (never v3 on-demand endpoints).
func TestOnypheOnlyAllowedPath(t *testing.T) {
	merklemapServe(t, &onypheBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v2/search/") {
			t.Errorf("forbidden path requested: %s", r.URL.Path)
			w.WriteHeader(500)
			return
		}
		merklemapWrite(w, merklemapFixture(t, "onyphe", "page1.json"))
	})
	if _, err := merklemapRun(onyphe{}, merklemapSession("onyphe", onypheCreds, 1)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(onypheAllowedPath, "v3") || strings.Contains(onypheAllowedPath, "ondemand") {
		t.Fatal("allowed path must be v2 search")
	}
}

func TestOnypheBattery(t *testing.T) {
	merklemapBattery(t, onyphe{}, &onypheBaseURL, onypheCreds, "<html>x</html>")
}

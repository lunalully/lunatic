package sources

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

var rsecloudCreds = map[string]string{"api_key": "secret-key"}

func TestRsecloudPagination(t *testing.T) {
	var pages []string
	merklemapServe(t, &rsecloudBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != rsecloudAllowedPath+"example.com" {
			t.Errorf("forbidden path %s", r.URL.Path)
			w.WriteHeader(500)
			return
		}
		if r.Header.Get("X-API-Key") != "secret-key" {
			t.Errorf("key header missing")
		}
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		merklemapWrite(w, merklemapFixture(t, "rsecloud", "page"+p+".json"))
	})
	got, err := merklemapRun(rsecloud{}, merklemapSession("rsecloud", rsecloudCreds, 10))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com", "mail.example.com")
	if !reflect.DeepEqual(pages, []string{"1", "2"}) {
		t.Fatalf("pages %v", pages)
	}
}

// Never touch the /active/ endpoint.
func TestRsecloudNeverActive(t *testing.T) {
	merklemapServe(t, &rsecloudBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "active") && !strings.Contains(r.URL.Path, "passive") {
			t.Errorf("active endpoint requested: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.URL.Path, "/api/v2/subdomains/passive/") {
			t.Errorf("path outside allowed prefix: %s", r.URL.Path)
		}
		merklemapWrite(w, merklemapFixture(t, "rsecloud", "empty.json"))
	})
	if _, err := merklemapRun(rsecloud{}, merklemapSession("rsecloud", rsecloudCreds, 3)); err != nil {
		t.Fatal(err)
	}
}

func TestRsecloudMaxPages(t *testing.T) {
	hits := merklemapServe(t, &rsecloudBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"data":["a.example.com"],"total_pages":99}`))
	})
	if _, err := merklemapRun(rsecloud{}, merklemapSession("rsecloud", rsecloudCreds, 3)); err != nil || *hits != 3 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestRsecloudEmpty(t *testing.T) {
	merklemapServe(t, &rsecloudBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "rsecloud", "empty.json"))
	})
	got, err := merklemapRun(rsecloud{}, merklemapSession("rsecloud", rsecloudCreds, 3))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestRsecloudErrorIn200(t *testing.T) {
	merklemapServe(t, &rsecloudBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "rsecloud", "error200.json"))
	})
	_, err := merklemapRun(rsecloud{}, merklemapSession("rsecloud", rsecloudCreds, 3))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestRsecloudBattery(t *testing.T) {
	merklemapBattery(t, rsecloud{}, &rsecloudBaseURL, rsecloudCreds, "<html>x</html>")
}

package sources

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

var redhuntlabsCreds = map[string]string{"api_key": "secret-key"}

func TestRedhuntlabsPagination(t *testing.T) {
	old := redhuntlabsPageSize
	redhuntlabsPageSize = 2
	defer func() { redhuntlabsPageSize = old }()
	var pages []string
	merklemapServe(t, &redhuntlabsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/community/v1/domains/subdomains" || r.URL.Query().Get("domain") != "example.com" {
			t.Errorf("url %s", r.URL)
		}
		if r.Header.Get("X-BLOBR-KEY") != "secret-key" {
			t.Errorf("key header missing")
		}
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		merklemapWrite(w, merklemapFixture(t, "redhuntlabs", "page"+p+".json"))
	})
	got, err := merklemapRun(redhuntlabs{}, merklemapSession("redhuntlabs", redhuntlabsCreds, 10))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com", "mail.example.com")
	if !reflect.DeepEqual(pages, []string{"1", "2"}) {
		t.Fatalf("pages %v", pages)
	}
}

func TestRedhuntlabsMaxPagesAndQuotaCap(t *testing.T) {
	hits := merklemapServe(t, &redhuntlabsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"subdomains":["a.example.com"],"metadata":{"result_count":999999,"page_size":1}}`))
	})
	// user MaxPages below the internal cap
	if _, err := merklemapRun(redhuntlabs{}, merklemapSession("redhuntlabs", redhuntlabsCreds, 2)); err != nil || *hits != 2 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
	// user MaxPages above the internal cap (100 requests/month plan)
	*hits = 0
	if _, err := merklemapRun(redhuntlabs{}, merklemapSession("redhuntlabs", redhuntlabsCreds, 50)); err != nil || int(*hits) != redhuntlabsMaxPages {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestRedhuntlabsEmpty(t *testing.T) {
	merklemapServe(t, &redhuntlabsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "redhuntlabs", "empty.json"))
	})
	got, err := merklemapRun(redhuntlabs{}, merklemapSession("redhuntlabs", redhuntlabsCreds, 3))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestRedhuntlabsErrorIn200(t *testing.T) {
	merklemapServe(t, &redhuntlabsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "redhuntlabs", "error200.json"))
	})
	_, err := merklemapRun(redhuntlabs{}, merklemapSession("redhuntlabs", redhuntlabsCreds, 3))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v", err)
	}
}

func TestRedhuntlabsBattery(t *testing.T) {
	merklemapBattery(t, redhuntlabs{}, &redhuntlabsBaseURL, redhuntlabsCreds, "<html>x</html>")
}

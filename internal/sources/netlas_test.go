package sources

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var netlasCreds = map[string]string{"api_key": "secret-key"}

func TestNetlasSuccess(t *testing.T) {
	var sawDownload bool
	merklemapServe(t, &netlasBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/api/domains_count/":
			if !strings.Contains(r.URL.Query().Get("q"), "domain:*.example.com") {
				t.Errorf("q %q", r.URL.Query().Get("q"))
			}
			merklemapWrite(w, merklemapFixture(t, "netlas", "count.json"))
		case "/api/domains/download/":
			sawDownload = true
			b, _ := io.ReadAll(r.Body)
			if r.Method != "POST" || !strings.Contains(string(b), `"size":3`) || !strings.Contains(string(b), "domain:*.example.com") {
				t.Errorf("body %s", b)
			}
			merklemapWrite(w, merklemapFixture(t, "netlas", "download.json"))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	got, err := merklemapRun(netlas{}, merklemapSession("netlas", netlasCreds, 5))
	merklemapExpect(t, got, err, "www.example.com", "mail.example.com", "api.example.com")
	if !sawDownload {
		t.Fatal("no download")
	}
}

func TestNetlasSizeCap(t *testing.T) {
	merklemapServe(t, &netlasBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/domains_count/" {
			merklemapWrite(w, []byte(`{"count":99999}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), `"size":200`) {
			t.Errorf("body %s", b)
		}
		merklemapWrite(w, []byte(`[]`))
	})
	if _, err := merklemapRun(netlas{}, merklemapSession("netlas", netlasCreds, 5)); err != nil {
		t.Fatal(err)
	}
}

func TestNetlasEmpty(t *testing.T) {
	hits := merklemapServe(t, &netlasBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "netlas", "count_zero.json"))
	})
	got, err := merklemapRun(netlas{}, merklemapSession("netlas", netlasCreds, 5))
	if err != nil || len(got) != 0 || *hits != 1 {
		t.Fatalf("%v %v %d", got, err, *hits)
	}
}

func TestNetlasErrorIn200(t *testing.T) {
	merklemapServe(t, &netlasBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "netlas", "error200.json"))
	})
	_, err := merklemapRun(netlas{}, merklemapSession("netlas", netlasCreds, 5))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestNetlasDownloadErrorIn200(t *testing.T) {
	merklemapServe(t, &netlasBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/domains_count/" {
			merklemapWrite(w, []byte(`{"count":5}`))
			return
		}
		merklemapWrite(w, []byte(`{"detail":"Request limit exceeded"}`))
	})
	_, err := merklemapRun(netlas{}, merklemapSession("netlas", netlasCreds, 5))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v", err)
	}
}

func TestNetlasBattery(t *testing.T) {
	merklemapBattery(t, netlas{}, &netlasBaseURL, netlasCreds, "not json")
}

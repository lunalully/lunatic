package sources

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var pugreconCreds = map[string]string{"api_key": "secret-key"}

func TestPugreconSuccess(t *testing.T) {
	merklemapServe(t, &pugreconBaseURL, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/api/v1/domains" || !strings.Contains(string(b), `"domain_name":"example.com"`) {
			t.Errorf("%s %s %s", r.Method, r.URL.Path, b)
		}
		if r.Header.Get("Authorization") != "Bearer secret-key" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		merklemapWrite(w, merklemapFixture(t, "pugrecon", "success.json"))
	})
	got, err := merklemapRun(pugrecon{}, merklemapSession("pugrecon", pugreconCreds, 1))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com")
}

func TestPugreconEmpty(t *testing.T) {
	merklemapServe(t, &pugreconBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "pugrecon", "empty.json"))
	})
	got, err := merklemapRun(pugrecon{}, merklemapSession("pugrecon", pugreconCreds, 1))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPugreconQuotaIn200(t *testing.T) {
	merklemapServe(t, &pugreconBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "pugrecon", "quota.json"))
	})
	_, err := merklemapRun(pugrecon{}, merklemapSession("pugrecon", pugreconCreds, 1))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v", err)
	}
}

func TestPugreconErrorField(t *testing.T) {
	merklemapServe(t, &pugreconBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"error":"unauthorized"}`))
	})
	_, err := merklemapRun(pugrecon{}, merklemapSession("pugrecon", pugreconCreds, 1))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestPugreconBattery(t *testing.T) {
	merklemapBattery(t, pugrecon{}, &pugreconBaseURL, pugreconCreds, "<html>bad gateway</html>")
}

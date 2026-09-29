package sources

import (
	"errors"
	"net/http"
	"testing"
)

var reconeerCreds = map[string]string{"api_key": "secret-key"}

func TestReconeerSuccess(t *testing.T) {
	merklemapServe(t, &reconeerBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/domain/example.com" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret-key" || r.URL.Query().Get("api_key") != "" {
			t.Errorf("credential must be in the Authorization header only")
		}
		merklemapWrite(w, merklemapFixture(t, "reconeer", "success.json"))
	})
	got, err := merklemapRun(reconeer{}, merklemapSession("reconeer", reconeerCreds, 1))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com")
}

func TestReconeerEmpty(t *testing.T) {
	merklemapServe(t, &reconeerBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "reconeer", "empty.json"))
	})
	got, err := merklemapRun(reconeer{}, merklemapSession("reconeer", reconeerCreds, 1))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestReconeer404IsNoResults(t *testing.T) {
	merklemapServe(t, &reconeerBaseURL, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	got, err := merklemapRun(reconeer{}, merklemapSession("reconeer", reconeerCreds, 1))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestReconeer402(t *testing.T) {
	merklemapServe(t, &reconeerBaseURL, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(402) })
	_, err := merklemapRun(reconeer{}, merklemapSession("reconeer", reconeerCreds, 1))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestReconeerErrorIn200(t *testing.T) {
	merklemapServe(t, &reconeerBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "reconeer", "error200.json"))
	})
	_, err := merklemapRun(reconeer{}, merklemapSession("reconeer", reconeerCreds, 1))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestReconeerBattery(t *testing.T) {
	merklemapBattery(t, reconeer{}, &reconeerBaseURL, reconeerCreds, "<html>x</html>")
}

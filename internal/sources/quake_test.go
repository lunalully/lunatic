package sources

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

var quakeCreds = map[string]string{"api_key": "secret-key"}

func TestQuakePagination(t *testing.T) {
	old := quakeSize
	quakeSize = 2
	defer func() { quakeSize = old }()
	var starts []float64
	merklemapServe(t, &quakeBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/v3/search/quake_service" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-QuakeToken") != "secret-key" {
			t.Errorf("token header missing")
		}
		b, _ := io.ReadAll(r.Body)
		var req map[string]any
		_ = json.Unmarshal(b, &req)
		if req["query"] != `domain: "example.com"` || req["size"].(float64) != 2 {
			t.Errorf("req %s", b)
		}
		st := req["start"].(float64)
		starts = append(starts, st)
		if st == 0 {
			merklemapWrite(w, merklemapFixture(t, "quake", "page1.json"))
		} else {
			merklemapWrite(w, merklemapFixture(t, "quake", "page2.json"))
		}
	})
	got, err := merklemapRun(quake{}, merklemapSession("quake", quakeCreds, 10))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com", "vpn.example.com")
	if len(starts) != 2 || starts[1] != 2 {
		t.Fatalf("starts %v", starts)
	}
}

func TestQuakeMaxPages(t *testing.T) {
	old := quakeSize
	quakeSize = 1
	defer func() { quakeSize = old }()
	hits := merklemapServe(t, &quakeBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"code":0,"data":[{"service":{"http":{"host":"a.example.com"}}}],"meta":{"pagination":{"total":1000}}}`))
	})
	if _, err := merklemapRun(quake{}, merklemapSession("quake", quakeCreds, 3)); err != nil || *hits != 3 {
		t.Fatalf("err=%v hits=%d", err, *hits)
	}
}

func TestQuakeEmpty(t *testing.T) {
	merklemapServe(t, &quakeBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "quake", "empty.json"))
	})
	got, err := merklemapRun(quake{}, merklemapSession("quake", quakeCreds, 3))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestQuakeErrorIn200(t *testing.T) {
	for file, want := range map[string]error{"error200.json": ErrAuth, "quota200.json": ErrRateLimited} {
		file, want := file, want
		t.Run(file, func(t *testing.T) {
			merklemapServe(t, &quakeBaseURL, func(w http.ResponseWriter, r *http.Request) {
				merklemapWrite(w, merklemapFixture(t, "quake", file))
			})
			_, err := merklemapRun(quake{}, merklemapSession("quake", quakeCreds, 3))
			if !errors.Is(err, want) {
				t.Fatalf("err=%v want %v", err, want)
			}
		})
	}
}

func TestQuakeUnknownCode(t *testing.T) {
	merklemapServe(t, &quakeBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, []byte(`{"code":"q9999","message":"something odd"}`))
	})
	_, err := merklemapRun(quake{}, merklemapSession("quake", quakeCreds, 3))
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func TestQuakeBattery(t *testing.T) {
	merklemapBattery(t, quake{}, &quakeBaseURL, quakeCreds, "<html>x</html>")
}

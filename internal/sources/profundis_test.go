package sources

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

var profundisCreds = map[string]string{"api_key": "secret-key"}

func TestProfundisStream(t *testing.T) {
	merklemapServe(t, &profundisBaseURL, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/api/v2/common/data/subdomains" || !strings.Contains(string(b), `"domain":"example.com"`) {
			t.Errorf("%s %s %s", r.Method, r.URL.Path, b)
		}
		if r.Header.Get("X-API-KEY") != "secret-key" || r.Header.Get("Accept") != "text/event-stream" {
			t.Errorf("headers %v", r.Header)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(merklemapFixture(t, "profundis", "stream.txt"))
	})
	got, err := merklemapRun(profundis{}, merklemapSession("profundis", profundisCreds, 1))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com", "mail.example.com")
}

func TestProfundisEmpty(t *testing.T) {
	merklemapServe(t, &profundisBaseURL, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(merklemapFixture(t, "profundis", "empty.txt"))
	})
	got, err := merklemapRun(profundis{}, merklemapSession("profundis", profundisCreds, 1))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestProfundisErrorIn200(t *testing.T) {
	merklemapServe(t, &profundisBaseURL, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(merklemapFixture(t, "profundis", "error.txt"))
	})
	_, err := merklemapRun(profundis{}, merklemapSession("profundis", profundisCreds, 1))
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("err=%v", err)
	}
}

func TestProfundisHTMLInvalid(t *testing.T) {
	merklemapServe(t, &profundisBaseURL, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>maintenance</body></html>"))
	})
	_, err := merklemapRun(profundis{}, merklemapSession("profundis", profundisCreds, 1))
	if !errors.Is(err, ErrUnexpected) {
		t.Fatalf("err=%v", err)
	}
}

func TestProfundisBattery(t *testing.T) {
	// plain-text protocol: the "invalid response" case is the HTML page above; use an error line here
	merklemapBattery(t, profundis{}, &profundisBaseURL, profundisCreds, "error: malformed request")
}

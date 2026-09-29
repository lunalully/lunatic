package sources

import (
	"errors"
	"net/http"
	"testing"
)

var securitytrailsCreds = map[string]string{"api_key": "secret-key"}

func TestSecuritytrailsSuccess(t *testing.T) {
	merklemapServe(t, &securitytrailsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/domain/example.com/subdomains" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("APIKEY") != "secret-key" || r.URL.Query().Get("apikey") != "" {
			t.Errorf("credential must be in the APIKEY header only")
		}
		merklemapWrite(w, merklemapFixture(t, "securitytrails", "success.json"))
	})
	got, err := merklemapRun(securitytrails{}, merklemapSession("securitytrails", securitytrailsCreds, 1))
	merklemapExpect(t, got, err, "www.example.com", "api.example.com", "mail.example.com")
}

func TestSecuritytrailsEmpty(t *testing.T) {
	merklemapServe(t, &securitytrailsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "securitytrails", "empty.json"))
	})
	got, err := merklemapRun(securitytrails{}, merklemapSession("securitytrails", securitytrailsCreds, 1))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestSecuritytrailsErrorIn200(t *testing.T) {
	merklemapServe(t, &securitytrailsBaseURL, func(w http.ResponseWriter, r *http.Request) {
		merklemapWrite(w, merklemapFixture(t, "securitytrails", "error200.json"))
	})
	_, err := merklemapRun(securitytrails{}, merklemapSession("securitytrails", securitytrailsCreds, 1))
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err=%v", err)
	}
}

func TestSecuritytrailsBattery(t *testing.T) {
	merklemapBattery(t, securitytrails{}, &securitytrailsBaseURL, securitytrailsCreds, "<html>x</html>")
}

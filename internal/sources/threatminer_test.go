package sources

import (
	"net/http"
	"testing"
)

func TestThreatminerSuccess(t *testing.T) {
	var q string
	got, err := g2run(t, threatminer{}, &threatminerBaseURL, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.String()
		w.Write(g2fixture(t, "threatminer", "success.json"))
	}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "mail.example.com", "OTHER.example.net")
	if q != "/v2/domain.php?q=example.com&rt=5" {
		t.Fatalf("url %q", q)
	}
}

func TestThreatminerNotFoundBody(t *testing.T) {
	got, err := g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, g2fixture(t, "threatminer", "notfound.json")), nil, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestThreatminerNotFoundHTTP(t *testing.T) {
	got, err := g2run(t, threatminer{}, &threatminerBaseURL, g2serve(404, []byte(`{"status_code":"404"}`)), nil, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestThreatminerNumericCodeAndEmpty(t *testing.T) {
	got, err := g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, []byte(`{"status_code":200,"results":["a.example.com"]}`)), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "a.example.com")
	got, err = g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, []byte(`{"status_code":"200","results":[]}`)), nil, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestThreatminerErrorInBody(t *testing.T) {
	_, err := g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, []byte(`{"status_code":"500","status_message":"boom"}`)), nil, 1)
	g2is(t, err, ErrUnexpected)
	_, err = g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, []byte(`{"status_code":"429","status_message":"limit"}`)), nil, 1)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, []byte(`{}`)), nil, 1)
	g2is(t, err, ErrUnexpected)
}

func TestThreatminerHTTPErrors(t *testing.T) {
	_, err := g2run(t, threatminer{}, &threatminerBaseURL, g2serve(429, []byte("x")), nil, 1)
	g2is(t, err, ErrRateLimited)
	_, err = g2run(t, threatminer{}, &threatminerBaseURL, g2serve(200, []byte("not json")), nil, 1)
	g2is(t, err, ErrUnexpected)
}

func TestThreatminerInfo(t *testing.T) {
	i := threatminer{}.Info()
	if !i.Default || i.Auth != AuthNone || i.RPS != 0.1 {
		t.Fatalf("%+v", i)
	}
}

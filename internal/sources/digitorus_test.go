package sources

import "testing"

func TestDigitorusParse(t *testing.T) {
	got, err := g2run(t, digitorus{}, &digitorusBaseURL, g2serve(200, g2fixture(t, "digitorus", "page.html")), nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	g2want(t, got, "www.example.com", "api.example.com", "dev.example.com")
	if i := (digitorus{}).Info(); i.Default || i.Auth != AuthNone {
		t.Fatalf("info %+v", i)
	}
}

func TestDigitorusEmpty(t *testing.T) {
	got, err := g2run(t, digitorus{}, &digitorusBaseURL, g2serve(200, []byte("<html><body>nothing</body></html>")), nil, 1)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestDigitorusChallenge(t *testing.T) {
	ch := g2fixture(t, "digitorus", "challenge.html")
	_, err := g2run(t, digitorus{}, &digitorusBaseURL, g2serve(200, ch), nil, 1)
	g2is(t, err, ErrUnavailable)
	_, err = g2run(t, digitorus{}, &digitorusBaseURL, g2serve(403, ch), nil, 1)
	g2is(t, err, ErrUnavailable)
	_, err = g2run(t, digitorus{}, &digitorusBaseURL, g2serve(429, nil), nil, 1)
	g2is(t, err, ErrRateLimited)
}

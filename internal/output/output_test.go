package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lunalully/lunatic/internal/runner"
)

var sample = []runner.Finding{
	{Domain: "example.com", Subdomain: "api.example.com", Sources: []string{"crtsh", "waybackarchive"}},
	{Domain: "example.com", Subdomain: "www.example.com", Sources: []string{"crtsh"}},
	{Domain: "dev.example.com", Subdomain: "www.example.com", Sources: []string{"x"}},
	{Domain: "example.org", Subdomain: "a.example.org", Sources: []string{"y"}},
}

func TestTXT(t *testing.T) {
	var b bytes.Buffer
	if err := WriteTXT(&b, sample); err != nil {
		t.Fatal(err)
	}
	want := "api.example.com\nwww.example.com\na.example.org\n"
	if b.String() != want {
		t.Fatalf("got %q", b.String())
	}
	b.Reset()
	WriteTXT(&b, nil)
	if b.Len() != 0 {
		t.Fatal("empty input must write nothing")
	}
}

func TestJSONL(t *testing.T) {
	var b bytes.Buffer
	if err := WriteJSONL(&b, sample[:2]); err != nil {
		t.Fatal(err)
	}
	want := `{"domain":"example.com","subdomain":"api.example.com","sources":["crtsh","waybackarchive"]}` + "\n" +
		`{"domain":"example.com","subdomain":"www.example.com","sources":["crtsh"]}` + "\n"
	if b.String() != want {
		t.Fatalf("got %q", b.String())
	}
	if strings.Contains(b.String(), "\x1b") {
		t.Fatal("ANSI")
	}
}

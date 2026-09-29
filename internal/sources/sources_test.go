package sources

import (
	"context"
	"errors"
	"testing"
)

type fake struct{ name string }

func (f fake) Info() Info { return Info{Name: f.name} }
func (f fake) Enumerate(context.Context, string, *Session, func(string)) error {
	return nil
}

func TestRegistry(t *testing.T) {
	Register(fake{"zz-b"})
	Register(fake{"zz-a"})
	if _, ok := Get("zz-a"); !ok {
		t.Fatal("Get failed")
	}
	all := All()
	if len(all) < 2 || all[len(all)-2].Info().Name != "zz-a" || all[len(all)-1].Info().Name != "zz-b" {
		t.Fatalf("not sorted: %v", all)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate must panic")
		}
	}()
	Register(fake{"zz-a"})
}

func TestPickKey(t *testing.T) {
	if k, err := PickKey(map[string]string{"api_key": "x"}, "api_key"); err != nil || k != "x" {
		t.Fatal(k, err)
	}
	if _, err := PickKey(map[string]string{"api_key": ""}, "api_key"); !errors.Is(err, ErrNoKey) {
		t.Fatal(err)
	}
	var s *Session
	s.Logf("nil-safe %d", 1)
}

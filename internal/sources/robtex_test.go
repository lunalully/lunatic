package sources

import (
	"context"
	"errors"
	"testing"
)

func TestRobtexDisabled(t *testing.T) {
	i := robtex{}.Info()
	if !i.Disabled || i.DisabledReason == "" {
		t.Fatalf("%+v", i)
	}
	err := robtex{}.Enumerate(context.Background(), "example.com", nil, func(string) { t.Fatal("emit") })
	if !errors.Is(err, ErrDisabled) {
		t.Fatalf("err=%v", err)
	}
}

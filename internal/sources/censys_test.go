package sources

import (
	"context"
	"errors"
	"testing"
)

func TestCensysDisabled(t *testing.T) {
	i := censys{}.Info()
	if !i.Disabled || i.DisabledReason == "" || i.Default {
		t.Fatalf("%+v", i)
	}
	if err := (censys{}).Enumerate(context.Background(), "example.com", &Session{}, func(string) {}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("%v", err)
	}
}

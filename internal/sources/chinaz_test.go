package sources

import (
	"context"
	"errors"
	"testing"
)

func TestChinazDisabled(t *testing.T) {
	i := chinaz{}.Info()
	if !i.Disabled || i.DisabledReason == "" || i.Default {
		t.Fatalf("%+v", i)
	}
	if err := (chinaz{}).Enumerate(context.Background(), "example.com", &Session{}, func(string) {}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("%v", err)
	}
}

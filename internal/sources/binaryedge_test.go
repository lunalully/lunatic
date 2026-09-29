package sources

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func Test_binaryedge_Disabled(t *testing.T) {
	s, ok := Get("binaryedge")
	if !ok {
		t.Fatal("not registered")
	}
	i := s.Info()
	if !i.Disabled || !strings.Contains(i.DisabledReason, "2025-03-31") {
		t.Fatalf("info: %+v", i)
	}
	if err := s.Enumerate(context.Background(), "example.com", nil, func(string) { t.Error("emitted") }); !errors.Is(err, ErrDisabled) {
		t.Fatalf("err=%v", err)
	}
}

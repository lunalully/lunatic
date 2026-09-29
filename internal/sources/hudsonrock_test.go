package sources

import (
	"strings"
	"testing"
)

func TestHudsonrockDisabled(t *testing.T) {
	g2disabled(t, hudsonrock{})
	if !strings.Contains((hudsonrock{}).Info().DisabledReason, "infostealer") {
		t.Fatal("reason should mention infostealer")
	}
}

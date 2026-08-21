package cmd

import (
	"strings"
	"testing"
)

// @ft:33
func TestVersion_PrintsNonEmptyVersion(t *testing.T) {
	out, err := runTour(t, "version")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if strings.TrimSpace(out) == "" {
		t.Errorf("expected a non-empty version string, got %q", out)
	}
}

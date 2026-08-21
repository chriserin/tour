package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

// @ft:25
func TestLaunch_StructuralErrorPreventsOpening(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "example.tour"), `Slide with no location line.
`)

	// This never reaches tea.NewProgram(...).Run() — runRoot returns before
	// that once parseErrs is non-empty, so this is safe to run as a normal
	// (non-interactive) test.
	out, err := runTour(t, filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}
	if !strings.Contains(out, "malformed @ file:line") && !strings.Contains(out, "missing @ file:line") {
		t.Errorf("expected the structural error to be reported, got:\n%s", out)
	}
}

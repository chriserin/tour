package viewer

import (
	"strings"
	"testing"
)

// This exists because the manual end-to-end smoke test caught something the
// rest of the suite couldn't: Model.renderCurrent silently falls back to
// raw text on a glamour error, so a test only checking that narration text
// appears in the view can't tell "rendered" from "glamour is broken and we
// fell back." This asserts markdown was actually transformed, not just that
// the words survived.
func TestRenderMarkdown(t *testing.T) {
	out, err := renderMarkdown("# Welcome\n\nThis is **bold**.\n", 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "# Welcome") {
		t.Errorf("expected the heading marker to be rendered away, got: %q", out)
	}
	if strings.Contains(out, "**bold**") {
		t.Errorf("expected bold markers to be rendered away, got: %q", out)
	}
	if !strings.Contains(out, "Welcome") || !strings.Contains(out, "bold") {
		t.Errorf("expected the actual words to survive rendering, got: %q", out)
	}
}

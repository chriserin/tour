package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"tour/tourfile"
)

// runTour executes the tour command tree in-process, capturing combined
// stdout/stderr, and returns it along with any error from execution.
func runTour(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// @ft:1
func TestParse_WellFormedTour(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib", "app.rb"), "line1\nline2\nline3\n")
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/app.rb:1
This code does x.
---
@ lib/app.rb:2
This code does y.
---
@ lib/app.rb:3
This code does z.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "3 slide(s)") {
		t.Errorf("expected slide count in output, got:\n%s", out)
	}
	for _, want := range []string{"lib/app.rb:1", "lib/app.rb:2", "lib/app.rb:3", "This code does x.", "This code does y.", "This code does z."} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

// @ft:2
func TestParse_MissingLocationLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib", "app.rb"), "line1\nline2\n")
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/app.rb:1
First slide.
---
This slide has no location line.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}
	if !strings.Contains(out, "line 4") {
		t.Errorf("expected error to name the tour file's own line number (4), got:\n%s", out)
	}
	if !strings.Contains(out, "slide 2") {
		t.Errorf("expected the malformed block to be counted and named as slide 2, got:\n%s", out)
	}
	if strings.Contains(out, "First slide.") {
		t.Errorf("expected the per-slide listing to be suppressed when an error is found, got:\n%s", out)
	}
	if !strings.Contains(out, "2 slide(s), 1 error(s)") {
		t.Errorf("expected the malformed block to still be counted as a slide (2 slide(s), 1 error(s)), got:\n%s", out)
	}
}

// @ft:3
func TestParse_MultipleStructuralErrorsAllReported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "example.tour"), `Slide one has no location line.
---
Slide two also has no location line.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}
	if n := strings.Count(out, "error:"); n != 2 {
		t.Errorf("expected 2 errors reported, got %d:\n%s", n, out)
	}
}

func TestParse_LocationErrorSlideNumberAccountsForEarlierMalformedBlocks(t *testing.T) {
	dir := t.TempDir()
	// Block 1 is malformed (no location line) and excluded from []Slide.
	// Block 2 is well-formed but points at a missing file. Its true
	// position in the tour is slide 2 — not slide 1, which is what a
	// naive index into only the successfully-parsed slides would report.
	writeFile(t, filepath.Join(dir, "example.tour"), `Malformed slide with no location line.
---
@ lib/missing.rb:1
References a missing file.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}
	if !strings.Contains(out, "slide 2 (lib/missing.rb:1)") {
		t.Errorf("expected the location error to report its true position (slide 2), got:\n%s", out)
	}
}

func TestParse_ErrorsOrderedBySlideNotByType(t *testing.T) {
	dir := t.TempDir()
	// Slide 2 has a location error (missing file); slide 3 has a
	// structural error (no location line). Ordering by type would print
	// all structural errors before all location errors — slide 3's
	// error before slide 2's — even though slide 2 comes first in the
	// tour.
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/app.rb:1
First slide, well-formed.
---
@ lib/missing.rb:1
Second slide, missing file.
---
Third slide, no location line.
`)
	writeFile(t, filepath.Join(dir, "lib", "app.rb"), "line1\n")

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}

	slide2 := strings.Index(out, "slide 2")
	slide3 := strings.Index(out, "slide 3")
	if slide2 == -1 || slide3 == -1 {
		t.Fatalf("expected both slide 2 and slide 3 errors, got:\n%s", out)
	}
	if slide2 > slide3 {
		t.Errorf("expected slide 2's error to print before slide 3's, got:\n%s", out)
	}
}

// @ft:4
func TestParse_MissingReferencedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/missing.rb:5
References a file that doesn't exist.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}
	if !strings.Contains(out, "slide 1") || !strings.Contains(out, "lib/missing.rb:5") {
		t.Errorf("expected location error naming slide 1 (lib/missing.rb:5), got:\n%s", out)
	}
}

// @ft:5
func TestParse_LineBeyondEndOfFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib", "app.rb"), "line1\nline2\n")
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/app.rb:99
Points past the end of the file.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err == nil {
		t.Fatalf("expected an error, got none\noutput:\n%s", out)
	}
	if !strings.Contains(out, "slide 1") || !strings.Contains(out, "lib/app.rb:99") {
		t.Errorf("expected location error naming slide 1 (lib/app.rb:99), got:\n%s", out)
	}
}

// @ft:6
func TestParse_LocationWithoutLineNumberDefaultsToOne(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib", "app.rb"), "line1\nline2\n")
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/app.rb
Whole-file slide, no line number.
`)

	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "lib/app.rb:1") {
		t.Errorf("expected slide to be listed at line 1, got:\n%s", out)
	}
}

// @ft:7
func TestParse_ResolvesRelativeToTourFileNotWorkingDirectory(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "lib", "app.rb"), "line1\nline2\n")
	writeFile(t, filepath.Join(repo, "tours", "example.tour"), `@ ../lib/app.rb:1
References the lib file relative to the tour file's own directory.
`)

	elsewhere := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	tourPath := filepath.Join(repo, "tours", "example.tour")
	out, err := runTour(t, "parse", tourPath)
	if err != nil {
		t.Fatalf("unexpected error resolving relative to tour file: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(out, "file not found") {
		t.Errorf("expected the reference to resolve relative to the tour file's directory, got:\n%s", out)
	}
}

// @ft:8
func TestParse_ListedLocationMatchesAsAuthored(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, "lib", "app", "auth.ex"), "line1\nline2\n")
	writeFile(t, filepath.Join(repo, "tours", "example.tour"), `@ ../lib/app/auth.ex:2
Narration.
`)

	elsewhere := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldwd)

	tourPath := filepath.Join(repo, "tours", "example.tour")
	out, err := runTour(t, "parse", tourPath)
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "../lib/app/auth.ex:2") {
		t.Errorf("expected the listed location to match what was authored (../lib/app/auth.ex:2), got:\n%s", out)
	}
	if strings.Contains(out, repo) {
		t.Errorf("expected the listed location to stay relative, not be absolutized, got:\n%s", out)
	}
}

// @ft:9
func TestParse_SlideLocationsAreColumnAligned(t *testing.T) {
	slides := []tourfile.Slide{
		{Index: 1, File: "lib/short.rb", Line: 1, Text: "Short path."},
		{Index: 2, File: "lib/a_much_longer_filename_here.rb", Line: 2, Text: "Long path."},
	}

	// No color forced here: alignment must hold in plain text too, since
	// that's what a piped/CI run (per @ft:12) actually looks like.
	out := formatSuccess(lipgloss.NewRenderer(&bytes.Buffer{}), slides)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")[1:] // drop the "N slide(s)" header
	if len(lines) != 2 {
		t.Fatalf("expected 2 slide lines, got %d:\n%s", len(lines), out)
	}
	col0 := strings.Index(lines[0], "Short path.")
	col1 := strings.Index(lines[1], "Long path.")
	if col0 == -1 || col1 == -1 {
		t.Fatalf("expected both previews present, got:\n%s", out)
	}
	if col0 != col1 {
		t.Errorf("expected previews to start in the same column (got %d and %d):\n%s", col0, col1, out)
	}
}

// @ft:10
func TestParse_SuccessLineIsBoldGreenOnATerminal(t *testing.T) {
	slides := []tourfile.Slide{{Index: 1, File: "lib/app.rb", Line: 1, Text: "Narration."}}

	renderer := lipgloss.NewRenderer(&bytes.Buffer{})
	renderer.SetColorProfile(termenv.ANSI)
	out := formatSuccess(renderer, slides)

	header := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(header, "\x1b[") {
		t.Fatalf("expected the slide count line to carry color codes, got: %q", header)
	}
	if !strings.Contains(header, "1;32") && !strings.Contains(header, "32;1") {
		t.Errorf("expected bold green (SGR 1;32 or 32;1), got: %q", header)
	}
}

// @ft:11
func TestParse_SlideLocationsColoredDistinctlyFromNarration(t *testing.T) {
	slides := []tourfile.Slide{{Index: 1, File: "lib/app.rb", Line: 1, Text: "Narration."}}

	renderer := lipgloss.NewRenderer(&bytes.Buffer{})
	renderer.SetColorProfile(termenv.ANSI)
	out := formatSuccess(renderer, slides)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	slideLine := lines[1]

	if !strings.Contains(slideLine, "\x1b[") {
		t.Fatalf("expected the location to carry color codes, got: %q", slideLine)
	}
	// The narration itself should not still be inside that color: a reset
	// (\x1b[0m, part of lipgloss's Render) must appear before "Narration.".
	resetIdx := strings.Index(slideLine, "\x1b[0m")
	narrIdx := strings.Index(slideLine, "Narration.")
	if resetIdx == -1 || narrIdx == -1 || resetIdx > narrIdx {
		t.Errorf("expected color to reset before the narration preview, got: %q", slideLine)
	}
}

// @ft:12
func TestParse_NoColorCodesWhenNotATerminal(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib", "app.rb"), "line1\n")
	writeFile(t, filepath.Join(dir, "example.tour"), `@ lib/app.rb:1
Narration.
`)

	// runTour captures output into a bytes.Buffer, which lipgloss's
	// auto-detection never treats as a terminal — this is the same
	// "not a terminal" case a pipe or CI capture hits in practice.
	out, err := runTour(t, "parse", filepath.Join(dir, "example.tour"))
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if strings.Contains(out, "\x1b[") {
		t.Errorf("expected no ANSI escape codes when stdout isn't a terminal, got: %q", out)
	}
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"tour/tourfile"
)

func init() {
	rootCmd.AddCommand(parseCmd)
}

var parseCmd = &cobra.Command{
	Use:           "parse <tour-file>",
	Short:         "Validate a tour file and list its slides",
	Args:          cobra.ExactArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runParse,
}

const previewRunes = 60

func runParse(cmd *cobra.Command, args []string) error {
	path := args[0]

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	slides, parseErrs := tourfile.Parse(f)
	locErrs := tourfile.ValidateLocations(filepath.Dir(path), slides)

	out := cmd.OutOrStdout()
	nErrs := len(parseErrs) + len(locErrs)

	if nErrs == 0 {
		fmt.Fprint(out, formatSuccess(lipgloss.NewRenderer(out), slides))
		return nil
	}

	// On error, the per-slide listing is suppressed entirely (per
	// design/PARSE.md's "Invalid" output shape) — a broken tour's report
	// is what's wrong, not what's fine. The slide count includes blocks
	// that failed to parse: a malformed block still occupies a slide
	// position in the tour, per tourfile.Slide.Index's numbering.
	totalSlides := len(slides) + len(parseErrs)
	style := lipgloss.NewRenderer(out).NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	fmt.Fprintf(out, "%d slide(s), %d error(s)\n", totalSlides, nErrs)
	for _, e := range mergeErrors(parseErrs, locErrs) {
		fmt.Fprintln(out, style.Render("error: "+e))
	}

	return ErrReported
}

// formatSuccess renders a valid tour's slide listing: a bold-green slide
// count, then each slide's location — in a distinct color from its
// narration, per design/PARSE.md's Coloring section — column-aligned to the
// widest location in the tour so every preview starts at the same column.
// Taking the renderer as a parameter (rather than deriving it from out
// internally) is what makes this testable independent of a real terminal:
// callers needing deterministic color output can pass a renderer with a
// forced color profile instead of one that's auto-detected from a writer
// that will never look like a TTY (a bytes.Buffer, in tests).
func formatSuccess(renderer *lipgloss.Renderer, slides []tourfile.Slide) string {
	successStyle := renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	locationStyle := renderer.NewStyle().Foreground(lipgloss.Color("6"))

	var b strings.Builder
	fmt.Fprintln(&b, successStyle.Render(fmt.Sprintf("%d slide(s)", len(slides))))

	locs := make([]string, len(slides))
	width := 0
	for i, s := range slides {
		locs[i] = fmt.Sprintf("%s:%d", s.File, s.Line)
		if n := utf8.RuneCountInString(locs[i]); n > width {
			width = n
		}
	}
	for i, s := range slides {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(locs[i]))
		fmt.Fprintf(&b, "%s%s  %s\n", locationStyle.Render(locs[i]), pad, preview(s.Text, previewRunes))
	}
	return b.String()
}

// mergeErrors combines structural and location errors into one list of
// messages, ordered by the slide each belongs to — the two kinds are
// reported interleaved in tour order, not grouped by type, so an author
// reading top to bottom sees problems in the same order the slides appear.
func mergeErrors(parseErrs []tourfile.ParseError, locErrs []tourfile.LocationError) []string {
	type entry struct {
		slide int
		text  string
	}

	entries := make([]entry, 0, len(parseErrs)+len(locErrs))
	for _, e := range parseErrs {
		entries = append(entries, entry{slide: e.Slide, text: e.Error()})
	}
	for _, e := range locErrs {
		entries = append(entries, entry{slide: e.Slide.Index, text: e.Error()})
	}

	sort.SliceStable(entries, func(i, j int) bool { return entries[i].slide < entries[j].slide })

	texts := make([]string, len(entries))
	for i, e := range entries {
		texts[i] = e.text
	}
	return texts
}

// preview returns the first line of text, truncated to at most maxRunes
// runes, with "..." appended whenever anything was cut — either because
// text continued past the first line, or the first line itself exceeded
// maxRunes.
func preview(text string, maxRunes int) string {
	truncated := false

	firstLine := text
	if idx := strings.IndexByte(text, '\n'); idx != -1 {
		firstLine = text[:idx]
		truncated = true
	}

	runes := []rune(firstLine)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
		truncated = true
	}

	result := string(runes)
	if truncated {
		result += "..."
	}
	return result
}

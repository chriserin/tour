package viewer

import glamour "charm.land/glamour/v2"

// renderMarkdown renders a slide's narration for the body, per
// design/VIEWER.md's Body section: the full Markdown language, not a
// subset, styled for a terminal of the given width.
//
// glamour renders to a fixed wrap width, so unlike the rest of the model
// this can't be computed once — it's called again whenever the pane
// resizes (see Model.renderCurrent).
//
// WithEnvironmentConfig, not a hardcoded style, picks the style: it
// respects $GLAMOUR_STYLE if the reviewer has set one, and otherwise
// falls back to glamour's own "dark" default — glamour v2 dropped the
// v1 "auto" style that detected the terminal's background itself, so
// there's no better default available than the one glamour picks on its
// own.
func renderMarkdown(text string, width int) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithEnvironmentConfig(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return "", err
	}
	return r.Render(text)
}

// Package viewer implements the tour slide viewer TUI, per design/VIEWER.md.
package viewer

import (
	"os"
	"path/filepath"

	"tour/tourfile"
)

// Load reads and validates a tour file, per design/VIEWER.md's Validation
// section: every slide's location is checked up front, not just the one
// about to be shown.
//
// Structural errors mean the tour has no reliable "slide 1" to fall back to
// — the caller should report them and not open the viewer, per the
// "Structural errors" case in design/VIEWER.md's Validation section.
//
// Location errors don't prevent loading: they're returned alongside the
// slides so the viewer can still open and mark the affected slides, per the
// "Location errors only" case in the same section.
func Load(path string) (slides []tourfile.Slide, parseErrs []tourfile.ParseError, locErrs []tourfile.LocationError, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	defer f.Close()

	slides, parseErrs = tourfile.Parse(f)
	if len(parseErrs) > 0 {
		return slides, parseErrs, nil, nil
	}

	locErrs = tourfile.ValidateLocations(filepath.Dir(path), slides)
	return slides, parseErrs, locErrs, nil
}

package tourfile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

// LocationError reports that a slide's file:line doesn't resolve.
type LocationError struct {
	Slide Slide
	Err   error
}

func (e LocationError) Error() string {
	return fmt.Sprintf("slide %d (%s:%d): %s", e.Slide.Index, e.Slide.File, e.Slide.Line, e.Err)
}

// ValidateLocations checks that every slide's file:line resolves, with File
// resolved relative to tourDir (the tour file's own directory, not the
// current working directory). Every slide is checked regardless of earlier
// failures, so the whole tour's locations are reported in one pass.
func ValidateLocations(tourDir string, slides []Slide) []LocationError {
	var errs []LocationError
	for _, s := range slides {
		abs := filepath.Join(tourDir, s.File)
		f, err := os.Open(abs)
		if err != nil {
			errs = append(errs, LocationError{Slide: s, Err: fmt.Errorf("file not found")})
			continue
		}

		count, err := countLinesUpTo(f, s.Line)
		f.Close()
		if err != nil {
			errs = append(errs, LocationError{Slide: s, Err: err})
			continue
		}
		if count < s.Line {
			errs = append(errs, LocationError{
				Slide: s,
				Err:   fmt.Errorf("only %d line(s), wanted line %d", count, s.Line),
			})
		}
	}
	return errs
}

// countLinesUpTo counts lines in f, stopping early once target lines have
// been seen — no need to read the rest of a large file.
func countLinesUpTo(f *os.File, target int) (int, error) {
	scanner := bufio.NewScanner(f)
	count := 0
	for count < target && scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		return count, err
	}
	return count, nil
}

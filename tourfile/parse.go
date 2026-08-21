// Package tourfile parses and validates .tour files, per design/FILE.md.
package tourfile

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Slide is one step of a tour: narration text plus the location it points
// at. Index is the slide's 1-based position among *all* blocks in the tour
// file, including any that failed to parse — a malformed block still has an
// unambiguous position in the tour, even though its content doesn't.
type Slide struct {
	Index int
	File  string
	Line  int
	Text  string
}

// ParseError is a structural problem found while parsing a tour file. Slide
// is the block's 1-based position among all blocks, the same numbering
// Slide.Index uses; Line is the tour file's own line number, kept alongside
// for pinpointing exactly where in the file to look.
type ParseError struct {
	Slide int
	Line  int
	Msg   string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("slide %d (line %d): %s", e.Slide, e.Line, e.Msg)
}

const separator = "---"

var locationRe = regexp.MustCompile(`^@ ([^:]+)(?::(\d+))?$`)

// Parse reads a tour file and returns its slides in order, along with every
// structural error found. It never stops at the first error — a malformed
// slide is skipped and parsing resumes at the next separator, so every
// problem in the file is reported in one pass. Every block between
// separators counts as a slide, whether or not it parsed successfully.
func Parse(r io.Reader) ([]Slide, []ParseError) {
	var slides []Slide
	var errs []ParseError

	var block []string
	blockStart := 1
	blockIndex := 0
	lineNum := 0

	flush := func() {
		if len(block) == 0 {
			return
		}
		blockIndex++
		slide, err := parseBlock(block, blockStart, blockIndex)
		if err != nil {
			errs = append(errs, *err)
		} else {
			slides = append(slides, *slide)
		}
		block = nil
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if line == separator {
			flush()
			blockStart = lineNum + 1
			continue
		}
		block = append(block, line)
	}
	flush()

	return slides, errs
}

func parseBlock(lines []string, blockStart, index int) (*Slide, *ParseError) {
	idx := 0
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}
	if idx == len(lines) {
		return nil, &ParseError{Slide: index, Line: blockStart, Msg: "missing @ file:line"}
	}

	locLine := lines[idx]
	locLineNum := blockStart + idx

	m := locationRe.FindStringSubmatch(locLine)
	if m == nil {
		return nil, &ParseError{Slide: index, Line: locLineNum, Msg: "malformed @ file:line"}
	}

	file := m[1]
	line := 1
	if m[2] != "" {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, &ParseError{Slide: index, Line: locLineNum, Msg: "line number too large"}
		}
		line = n
	}

	text := strings.TrimSpace(strings.Join(lines[idx+1:], "\n"))

	return &Slide{Index: index, File: file, Line: line, Text: text}, nil
}

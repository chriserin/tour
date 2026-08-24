package viewer

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"tour/tourfile"
)

// ansiRe strips ANSI SGR escape codes so tests can assert on what a viewer
// actually reads on screen. glamour styles each word of wrapped narration
// with its own escape sequence, so a phrase like "slide one." is not a
// contiguous byte run in the raw rendered output even though it reads as
// one phrase visually — checking the raw bytes would be checking an
// implementation detail of glamour's wrapping, not what's shown.
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

func plain(s string) string { return ansiRe.ReplaceAllString(s, "") }

type fakeNvim struct {
	jumps []fakeJump
	err   error
}

type fakeJump struct {
	file   string
	line   int
	scroll *int
}

func (f *fakeNvim) Jump(file string, line int, scroll *int) error {
	f.jumps = append(f.jumps, fakeJump{file, line, scroll})
	return f.err
}

func slides(n int) []tourfile.Slide {
	s := make([]tourfile.Slide, n)
	for i := range s {
		s[i] = tourfile.Slide{
			Index: i + 1,
			File:  "lib/app.rb",
			Line:  i + 1,
			Text:  narrationFor(i + 1),
		}
	}
	return s
}

func narrationFor(n int) string {
	switch n {
	case 1:
		return "Narration for slide one."
	case 2:
		return "Narration for slide two."
	default:
		return "Narration for another slide."
	}
}

// sized runs the model through an initial WindowSizeMsg, which is what
// gives titleBar/statusBar/viewport a width to render against — real
// Bubble Tea programs deliver this automatically at startup.
func sized(m Model) Model {
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return updated.(Model)
}

func key(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func press(m Model, msg tea.KeyPressMsg) Model {
	updated, cmd := m.Update(msg)
	m = updated.(Model)
	if cmd != nil {
		if resultMsg := cmd(); resultMsg != nil {
			updated, _ = m.Update(resultMsg)
			m = updated.(Model)
		}
	}
	return m
}

func newTestModel(n int, nvim NvimClient) Model {
	var nvimErr error
	if nvim == nil {
		nvimErr = ErrNoNvimSession
	}
	m := New("test.tour", "/tour/dir", slides(n), nil, nvim, nvimErr)
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	return sized(m)
}

// @ft:13
func TestLaunchingShowsFirstSlide(t *testing.T) {
	m := newTestModel(3, &fakeNvim{})

	view := plain(m.View().Content)
	if !strings.Contains(view, "Narration for slide one.") {
		t.Errorf("expected slide 1's narration, got:\n%s", view)
	}
	if !strings.Contains(view, "1 of 3") {
		t.Errorf("expected position \"1 of 3\", got:\n%s", view)
	}
}

// @ft:14
func TestAdvancingToNextSlide(t *testing.T) {
	m := newTestModel(3, &fakeNvim{})
	m = press(m, key('n'))

	view := plain(m.View().Content)
	if !strings.Contains(view, "Narration for slide two.") {
		t.Errorf("expected slide 2's narration, got:\n%s", view)
	}
	if !strings.Contains(view, "2 of 3") {
		t.Errorf("expected position \"2 of 3\", got:\n%s", view)
	}
}

// @ft:15
func TestGoingBackToPreviousSlide(t *testing.T) {
	m := newTestModel(3, &fakeNvim{})
	m = press(m, key('n')) // slide 2
	m = press(m, key('p')) // back to slide 1

	view := plain(m.View().Content)
	if !strings.Contains(view, "Narration for slide one.") {
		t.Errorf("expected slide 1's narration, got:\n%s", view)
	}
	if !strings.Contains(view, "1 of 3") {
		t.Errorf("expected position \"1 of 3\", got:\n%s", view)
	}
}

// @ft:16
func TestSteppingPastLastSlideIsANoOp(t *testing.T) {
	m := newTestModel(2, &fakeNvim{})
	m = press(m, key('n')) // slide 2, the last one
	m = press(m, key('n')) // no-op

	view := plain(m.View().Content)
	if !strings.Contains(view, "2 of 2") {
		t.Errorf("expected to still be on the last slide (2 of 2), got:\n%s", view)
	}
}

// @ft:17
func TestSteppingBeforeFirstSlideIsANoOp(t *testing.T) {
	m := newTestModel(2, &fakeNvim{})
	m = press(m, key('p')) // no-op, already on slide 1

	view := plain(m.View().Content)
	if !strings.Contains(view, "1 of 2") {
		t.Errorf("expected to still be on the first slide (1 of 2), got:\n%s", view)
	}
}

// @ft:18
func TestJumpingDirectlyToASlideNumber(t *testing.T) {
	nvim := &fakeNvim{}
	m := newTestModel(9, nvim)

	m = press(m, key('g'))
	m = press(m, key('5'))
	m = press(m, key(tea.KeyEnter))

	view := plain(m.View().Content)
	if !strings.Contains(view, "5 of 9") {
		t.Errorf("expected to land directly on slide 5, got:\n%s", view)
	}
	// Only two jumps should have happened: the initial one to slide 1, and
	// the direct one to slide 5 — never stepping through 2, 3, and 4.
	if len(nvim.jumps) != 2 {
		t.Errorf("expected exactly 2 jumps (initial + direct), got %d: %+v", len(nvim.jumps), nvim.jumps)
	}
}

// @ft:19
func TestJumpingToFirstSlide(t *testing.T) {
	m := newTestModel(5, &fakeNvim{})
	m = press(m, key('G')) // last slide
	m = press(m, key('g'))
	m = press(m, key('g')) // gg -> first slide

	view := plain(m.View().Content)
	if !strings.Contains(view, "1 of 5") {
		t.Errorf("expected to be back on the first slide, got:\n%s", view)
	}
}

// @ft:20
func TestJumpingToLastSlide(t *testing.T) {
	m := newTestModel(5, &fakeNvim{})
	m = press(m, key('G'))

	view := plain(m.View().Content)
	if !strings.Contains(view, "5 of 5") {
		t.Errorf("expected to be on the last slide, got:\n%s", view)
	}
}

// @ft:21
func TestScrollingDoesNotChangeCurrentSlide(t *testing.T) {
	m := newTestModel(3, &fakeNvim{})
	m = press(m, key('n')) // slide 2

	before := plain(m.View().Content)
	m = press(m, key('j')) // scroll down
	after := plain(m.View().Content)

	if !strings.Contains(before, "2 of 3") || !strings.Contains(after, "2 of 3") {
		t.Errorf("expected to remain on slide 2 of 3 across scroll, before:\n%s\nafter:\n%s", before, after)
	}
}

// @ft:22
func TestScrollPositionResetsOnSlideChange(t *testing.T) {
	longNarration := strings.Repeat("This narration has many lines.\n\n", 40)
	s := []tourfile.Slide{
		{Index: 1, File: "lib/app.rb", Line: 1, Text: longNarration},
		{Index: 2, File: "lib/app.rb", Line: 2, Text: "Short."},
	}
	m := New("test.tour", "/tour/dir", s, nil, &fakeNvim{}, nil)
	m = sized(m)

	m = press(m, key('j')) // scroll down on slide 1
	if m.viewport.YOffset() == 0 {
		t.Fatalf("expected the scroll-down key to actually move the viewport for this fixture")
	}

	m = press(m, key('n')) // advance to slide 2
	if m.viewport.YOffset() != 0 {
		t.Errorf("expected scroll position to reset to top on slide change, got yOffset=%d", m.viewport.YOffset())
	}
}

// @ft:23
func TestAdvancingJumpsNeovimToItsLocation(t *testing.T) {
	nvim := &fakeNvim{}
	s := []tourfile.Slide{
		{Index: 1, File: "lib/first.rb", Line: 1, Text: "First."},
		{Index: 2, File: "lib/app.rb", Line: 42, Text: "Second."},
	}
	m := New("test.tour", "/tour/dir", s, nil, nvim, nil)
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	m = sized(m)

	m = press(m, key('n')) // advance to the lib/app.rb:42 slide

	if len(nvim.jumps) != 2 {
		t.Fatalf("expected 2 jumps (initial + advance), got %d: %+v", len(nvim.jumps), nvim.jumps)
	}
	last := nvim.jumps[len(nvim.jumps)-1]
	if !strings.HasSuffix(last.file, "lib/app.rb") || last.line != 42 {
		t.Errorf("expected the jump to target lib/app.rb:42, got %+v", last)
	}
}

// @ft:39
func TestAdvancingToAScrollOffsetSlidePositionsTheWindow(t *testing.T) {
	nvim := &fakeNvim{}
	scroll := 4
	s := []tourfile.Slide{
		{Index: 1, File: "lib/first.rb", Line: 1, Text: "First."},
		{Index: 2, File: "lib/app.rb", Line: 42, Scroll: &scroll, Text: "Second."},
	}
	m := New("test.tour", "/tour/dir", s, nil, nvim, nil)
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	m = sized(m)

	m = press(m, key('n')) // advance to the scroll-offset slide

	last := nvim.jumps[len(nvim.jumps)-1]
	if last.scroll == nil || *last.scroll != 4 {
		t.Errorf("expected the jump to carry a scroll offset of 4, got %+v", last)
	}
}

// @ft:40
func TestAdvancingWithNoScrollOffsetLeavesNeovimDefaultAlone(t *testing.T) {
	nvim := &fakeNvim{}
	s := []tourfile.Slide{
		{Index: 1, File: "lib/first.rb", Line: 1, Text: "First."},
		{Index: 2, File: "lib/app.rb", Line: 42, Text: "Second."},
	}
	m := New("test.tour", "/tour/dir", s, nil, nvim, nil)
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	m = sized(m)

	m = press(m, key('n')) // advance to the slide with no scroll offset

	last := nvim.jumps[len(nvim.jumps)-1]
	if last.scroll != nil {
		t.Errorf("expected no scroll offset to be sent, got %+v", last)
	}
}

// @ft:24
func TestNoNvimSessionStillShowsNarration(t *testing.T) {
	m := newTestModel(2, nil) // nil client: no session found

	view := plain(m.View().Content)
	if !strings.Contains(view, "Narration for slide one.") {
		t.Errorf("expected narration to still be shown, got:\n%s", view)
	}
	if !strings.Contains(view, ErrNoNvimSession.Error()) {
		t.Errorf("expected the status line to clearly indicate no Neovim session was found, got:\n%s", view)
	}
}

// @ft:26
func TestLocationErrorDoesNotPreventOpening(t *testing.T) {
	s := slides(3)
	locErr := errors.New("file not found")
	locErrs := []tourfile.LocationError{{Slide: s[1], Err: locErr}} // slide 2

	m := New("test.tour", "/tour/dir", s, locErrs, &fakeNvim{}, nil)
	if cmd := m.Init(); cmd != nil {
		if msg := cmd(); msg != nil {
			updated, _ := m.Update(msg)
			m = updated.(Model)
		}
	}
	m = sized(m)

	// The viewer opened normally, on slide 1.
	if !strings.Contains(plain(m.View().Content), "1 of 3") {
		t.Fatalf("expected the viewer to open normally despite the location error, got:\n%s", plain(m.View().Content))
	}

	m = press(m, key('n')) // land on the affected slide
	view := plain(m.View().Content)
	if !strings.Contains(view, "file not found") {
		t.Errorf("expected the status line to show the location error instead of file:line, got:\n%s", view)
	}
	if strings.Contains(view, "lib/app.rb:2") {
		t.Errorf("expected file:line to be replaced by the error, not shown alongside it, got:\n%s", view)
	}
}

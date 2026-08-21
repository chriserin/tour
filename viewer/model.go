package viewer

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"tour/tourfile"
)

// Model is the Bubble Tea model for the slide viewer, per design/VIEWER.md.
type Model struct {
	tourName string
	tourDir  string
	slides   []tourfile.Slide
	current  int // 0-based index into slides

	locErrs      map[int]error // keyed by Slide.Index
	nvim         NvimClient    // nil if no session was found/connected
	noSessionErr error         // why nvim is nil, e.g. ErrNoNvimSession
	nvimErr      error         // what the status line shows for the current slide

	viewport viewport.Model
	width    int
	height   int

	pendingG      bool
	pendingDigits string
}

// Body margin: the narration in the viewport gets breathing room on every
// side rather than sitting flush against the title/status bars and the
// terminal edges.
const (
	bodyMarginX = 2
	bodyMarginY = 1
)

var (
	// 256-color indices, not the base 16 ANSI colors: terminal themes
	// commonly remap the 16-color palette (color "4" rendered pale enough
	// in some themes for white text to lose contrast against it), but
	// leave the 256-color range alone, so these render consistently.
	titleBarStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("25"))
	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
	bodyStyle      = lipgloss.NewStyle().Padding(bodyMarginY, bodyMarginX)
)

// New constructs the viewer's model. nvim is nil if no Neovim session could
// be located/connected; nvimErr is the reason why, shown on the status line
// per design/VIEWER.md's "No Neovim session found" failure mode.
func New(tourName, tourDir string, slides []tourfile.Slide, locErrs []tourfile.LocationError, nvim NvimClient, nvimErr error) Model {
	errs := make(map[int]error, len(locErrs))
	for _, e := range locErrs {
		errs[e.Slide.Index] = e.Err
	}

	m := Model{
		tourName:     tourName,
		tourDir:      tourDir,
		slides:       slides,
		locErrs:      errs,
		nvim:         nvim,
		noSessionErr: nvimErr,
		viewport:     viewport.New(),
	}
	if len(slides) > 0 {
		m.gotoSlide(0) // sets current/nvimErr for slide 0; the Cmd it
		// returns is discarded here since Init(), not New(), is
		// responsible for handing Bubble Tea the initial Cmd to run.
	}
	return m
}

// Init fires the jump for the first slide, if there's a Neovim session to
// jump in and the first slide's own location isn't already known to be bad.
func (m Model) Init() tea.Cmd {
	if len(m.slides) == 0 {
		return nil
	}
	cp := m
	return cp.gotoSlide(0)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.viewport.SetWidth(m.bodyWidth())
		m.viewport.SetHeight(m.bodyHeight())
		m.renderCurrent()
		return m, nil

	case jumpResultMsg:
		m.nvimErr = msg.err
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pendingG {
		m.pendingG = false
		switch {
		case key == "g" && m.pendingDigits == "":
			return m, m.gotoSlide(0)
		case len(key) == 1 && key[0] >= '0' && key[0] <= '9':
			m.pendingDigits += key
			m.pendingG = true
			return m, nil
		case key == "enter" && m.pendingDigits != "":
			n, err := strconv.Atoi(m.pendingDigits)
			m.pendingDigits = ""
			if err != nil || n < 1 || n > len(m.slides) {
				return m, nil
			}
			return m, m.gotoSlide(n - 1)
		default:
			// Any other key cancels the pending g-sequence.
			m.pendingDigits = ""
			return m, nil
		}
	}

	switch key {
	case "n", "space", "l", "right":
		if m.current < len(m.slides)-1 {
			return m, m.gotoSlide(m.current + 1)
		}
	case "p", "h", "left":
		if m.current > 0 {
			return m, m.gotoSlide(m.current - 1)
		}
	case "g":
		m.pendingG = true
		m.pendingDigits = ""
	case "G":
		return m, m.gotoSlide(len(m.slides) - 1)
	case "j", "down":
		m.viewport.ScrollDown(1)
	case "k", "up":
		m.viewport.ScrollUp(1)
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

// gotoSlide switches to slide idx: resets scroll to the top (per design's
// "Scroll position resets ... on every slide change"), re-renders the body,
// and either records a known location error or fires a fresh jump — never
// leaving nvimErr holding a stale value from whatever slide was showing
// before.
func (m *Model) gotoSlide(idx int) tea.Cmd {
	m.current = idx
	m.viewport.GotoTop()
	m.renderCurrent()

	slide := m.slides[idx]
	if err, ok := m.locErrs[slide.Index]; ok {
		m.nvimErr = err
		return nil
	}
	if m.nvim == nil {
		m.nvimErr = m.noSessionErr
		return nil
	}
	m.nvimErr = nil
	return m.jumpCmd(idx)
}

type jumpResultMsg struct{ err error }

// jumpCmd resolves the slide's location to an absolute path (per
// implementation/VIEWER.md's Jump target section — Neovim's own working
// directory has no defined relationship to tour's or the tour file's) and
// asks nvim to jump there, as a tea.Cmd so a slow RPC call doesn't block
// the UI from redrawing.
func (m Model) jumpCmd(idx int) tea.Cmd {
	slide := m.slides[idx]
	abs, err := filepath.Abs(filepath.Join(m.tourDir, slide.File))
	if err != nil {
		return func() tea.Msg { return jumpResultMsg{err: err} }
	}
	nv := m.nvim
	line := slide.Line
	return func() tea.Msg {
		return jumpResultMsg{err: nv.Jump(abs, line)}
	}
}

func (m *Model) renderCurrent() {
	if m.width <= 0 || len(m.slides) == 0 {
		return
	}
	body, err := renderMarkdown(m.slides[m.current].Text, m.viewport.Width())
	if err != nil {
		body = m.slides[m.current].Text
	}
	m.viewport.SetContent(body)
}

// bodyWidth/bodyHeight are the viewport's own dimensions — the full
// terminal size minus the two bars and the body's margin on every side.
func (m Model) bodyWidth() int {
	w := m.width - 2*bodyMarginX
	if w < 1 {
		w = 1
	}
	return w
}

func (m Model) bodyHeight() int {
	h := m.height - 2 - 2*bodyMarginY // one line each for the title/status bars
	if h < 1 {
		h = 1
	}
	return h
}

func (m Model) View() tea.View {
	if len(m.slides) == 0 {
		return tea.NewView("")
	}
	body := bodyStyle.Render(m.viewport.View())
	v := tea.NewView(m.titleBar() + "\n" + body + "\n" + m.statusBar())
	// Alt screen: the viewer's frames live in a separate screen buffer, so
	// quitting restores the terminal exactly as it was before launch —
	// without this, every frame the TUI ever drew is left behind in the
	// scrollback history after exit.
	v.AltScreen = true
	return v
}

// titleBar shows the *code* location the current slide points at — just the
// file's basename, not its full path, since the title bar is prime real
// estate and a directory prefix is rarely what a reviewer needs at a
// glance — alongside the tour's position. It's replaced by the location
// error's text when the current slide's jump is known to fail, the same
// way file:line was replaced before this bar held it.
func (m Model) titleBar() string {
	left := m.currentLocationText()
	slideProgressText := fmt.Sprintf("slide %d of %d", m.current+1, len(m.slides))
	return titleBarStyle.Render(" " + justify(left, slideProgressText, m.width-2) + " ")
}

func (m Model) currentLocationText() string {
	if m.nvimErr != nil {
		return m.nvimErr.Error()
	}
	slide := m.slides[m.current]
	return fmt.Sprintf("%s:%d", filepath.Base(slide.File), slide.Line)
}

// statusBar carries the tour file's own name (moved down here to make room
// for the code location above) and the key hints.
func (m Model) statusBar() string {
	hints := "n/space next  p prev  q quit"
	return statusBarStyle.Render(" " + justify(m.tourName, hints, m.width-2) + " ")
}

// justify places left flush left and right flush right within width,
// separated by at least one space.
func justify(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

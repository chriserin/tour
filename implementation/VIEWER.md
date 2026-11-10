# Slide Viewer

Implements [design/VIEWER.md](../design/VIEWER.md). Built on
`charm.land/bubbletea/v2` (v2.0.9), `charm.land/bubbles/v2`,
`charm.land/lipgloss/v2`, `charm.land/glamour/v2`, and
`github.com/neovim/go-client`, Go 1.27.0.

**Import path note**: Charm's v2 packages (Bubble Tea, Bubbles, Lip
Gloss, Glamour) have moved their canonical module path to `charm.land/…`;
`github.com/charmbracelet/…` still resolves for v1, but a v2 import must
use the `charm.land` path or `go get` reports "module declares its path
as… but was required as…". `cmd/parse.go` still uses the older
`github.com/charmbracelet/lipgloss` v1 for its (non-interactive) color
output — the two coexist in the module graph fine, as unrelated modules
with different import paths, since `cmd/parse.go` has no reason to pull
in the rest of the v2 TUI stack.

**A version-skew build error worth knowing about**: pulling in the v2
stack alongside the existing v1 `lipgloss` dependency produced a build
failure in `github.com/charmbracelet/x/cellbuf` (`not enough arguments
in call to b.Italic`, etc.) — v1 `lipgloss` pins an old `cellbuf`
pre-release that predates an `x/ansi` API change the v2 stack pulls in
at a newer version, and Go's MVS can't paper over a genuine source
incompatibility between them. Fixed with `go get
github.com/charmbracelet/x/cellbuf@v0.0.15` to force the newer,
compatible release.

## TUI framework

Bubble Tea's Elm-architecture (`Init`/`Update`/`View`, immutable model,
messages) is a given per the user's direction, not an open choice. What's
still open is how much of the Charm ecosystem to lean on beyond the core
framework.

- **`bubbles/viewport`** (from `charmbracelet/bubbles`) for the body:
  handles line-wrapping and scroll offset, which the design's `j`/`k`
  scrolling and "does not spill into the title/status chrome" behavior
  both need. Writing this by hand would be re-implementing a solved,
  well-tested piece of the same ecosystem for no design-driven reason.
- **`lipgloss`** for layout and styling (borders, the title/status bars
  in the "what the user sees" mockup, bold/italic text styling). Same
  reasoning — it's the layout primitive Bubble Tea apps are built
  against, not an extra dependency beyond what Bubble Tea already
  implies.

## Markdown rendering

Design requires the full Markdown language rendered in the body —
headers, lists, tables, links, code blocks, blockquotes, and inline
emphasis/code spans — not a subset (see design's Body section for why
an earlier narrower version was dropped).

Options considered:

1. **`glamour`** (terminal Markdown renderer built on goldmark, with the
   GFM extension enabled by default) — parses full CommonMark + GFM
   (tables, autolinks, strikethrough) and renders it already styled for
   a terminal: headers as bold/underlined text, lists as indented
   bullets, code blocks with syntax highlighting via `chroma`, tables
   laid out with box-drawing characters, links shown inline.
2. **`goldmark` directly, with a custom `renderer.NodeRenderer`** —
   parse with goldmark's full default parser and hand-write a renderer
   for every node type it can produce (`Heading`, `List`, `ListItem`,
   `Table`, `Link`, `FencedCodeBlock`, `Blockquote`, ..., in addition to
   `Text`/`Emphasis`/`CodeSpan`).
3. **Hand-written inline scanner** — out of scope now that block-level
   constructs need to render too; a scanner never handled those.

**Decision: `glamour`.** An earlier version of this doc chose a
narrowly-configured goldmark parser specifically *because* only an
inline subset was in scope — headings and tables were deliberately
unrecognized so they'd fall through as literal text. Now that the full
language is in scope, that reason for avoiding glamour is gone, and
hand-writing a `NodeRenderer` covering every CommonMark + GFM node type
(option 2) means re-solving a problem glamour already solves well —
particularly code block syntax highlighting and table layout, both of
which are nontrivial on their own. This is the same reasoning that
already justified `bubbles/viewport` and `lipgloss` over hand-rolled
equivalents: use the maintained component unless the design specifically
requires behavior it can't produce, which is no longer the case here.

### Resizing

Unlike the narrow-subset version, `glamour.TermRenderer` renders to a
fixed wrap width (`glamour.WithWordWrap(width)`) rather than producing
width-independent styled spans, so the body's rendered output can't just
be computed once at load. The renderer is (re)constructed and the
current slide re-rendered whenever the body's width changes — on the
initial `tea.WindowSizeMsg` and on any subsequent resize — the same
pattern used by `glow` (Charm's own Markdown pager) for this exact
problem. The rendered string is then handed to `viewport.SetContent`
as before.

### Style selection

Glamour v2 dropped v1's `"auto"` standard style (which detected the
terminal's light/dark background itself) —
`glamour.WithStandardStyle("auto")` fails at construction with `auto:
style not found`; the only names registered are `ascii`, `dark`,
`light`, `notty`, `pink`, `dracula`, and `tokyo-night`. Used
`glamour.WithEnvironmentConfig()` instead, which respects
`$GLAMOUR_STYLE` if a reviewer has set one and otherwise falls back to
glamour's own `dark` default — there's no better default available now
that v2 doesn't do the detection itself, and this avoids hand-picking
one that would just be glamour's own fallback anyway.

### Testing

Deep byte-for-byte assertions on glamour's ANSI output would be brittle
and would mostly be testing glamour itself rather than this project's
code. `TestRenderMarkdown` (`viewer/markdown_test.go`) instead asserts
on the *transformation*: feed in `# Welcome` and `**bold**`, assert the
literal `#`/`**` markers are gone from the output and the words
survived — proof rendering actually ran, not just that the words are
present (see the next paragraph for why that distinction turned out to
matter). Pixel/ANSI-exact snapshot testing is treated as a non-goal.

This distinction was not theoretical: a manual end-to-end smoke test
(a real `tour` binary against a real `nvim --listen … --headless` in a
scratch tmux session) caught the `WithStandardStyle("auto")` bug above,
*after* the full `Model` test suite was green — because
`Model.renderCurrent` falls back to unrendered raw text on any glamour
error (deliberately, so a rendering failure doesn't blank the screen),
and every `Model` test up to that point only asserted that narration
*words* appeared in `View().Content`, which is true whether or not
glamour actually ran. `TestRenderMarkdown` closes that gap directly
against `renderMarkdown`, independent of the fallback.

Model-level view assertions have a second, unrelated wrinkle: glamour
wraps *each word* of rendered narration in its own SGR escape sequence,
so a phrase like "slide one." is not a contiguous byte run in
`View().Content` even though it reads as one phrase on screen —
`viewer/model_test.go`'s `plain()` helper strips ANSI codes before any
`strings.Contains` check, matching what a viewer actually reads rather
than glamour's wrapping as an implementation detail.

## Neovim RPC client

Options considered:

1. **`github.com/neovim/go-client`** — the official Go msgpack-RPC
   client for Neovim, used by `nvim`-Go plugin ecosystems.
2. **Hand-written msgpack-RPC over the Unix socket** — implement just
   the handful of calls needed (`nvim_get_current_win`, `nvim_command`
   or equivalent) directly against `net.Conn` and a msgpack encoder.

**Decision: `neovim/go-client`.** msgpack-RPC connection setup, request
IDs, and response correlation are exactly the kind of undifferentiated
protocol work a maintained client exists to remove; hand-rolling it
would be reimplementing the client for no benefit specific to this
project. The dependency surface is small and single-purpose (just the
RPC client), unlike pulling in a general framework for a narrow need.

### Jump target

Per design's "Jump target" decision (currently active window, no
attempt to manage layout): a small Lua script, run via `Nvim.ExecLua`,
equivalent to `:edit <file> | call cursor(<line>, 1)`. `:edit` operates
on the current window by default, and Neovim reuses an already-open
buffer for that file rather than reloading it — so "open if not already
open, otherwise just move the cursor" falls out of using `:edit`
directly rather than needing an explicit open-or-jump branch in `tour`'s
own code (verified directly against a real Neovim by
`TestNvimIntegration_JumpReusesAlreadyOpenBuffer`, per Testing below).

`ExecLua` takes `file` and `line` as real Lua varargs (`local file, line
= ...`) rather than being string-interpolated into the script — this
avoids needing to hand-escape a file path for Ex-command syntax
altogether (spaces, quotes, etc. are just msgpack-RPC argument values,
not text the Lua/Ex parser has to tokenize):

```lua
local file, line = ...
vim.cmd.edit(vim.fn.fnameescape(file))
vim.api.nvim_win_set_cursor(0, {line, 0})
```

`<file>` sent to Neovim must be an **absolute** path — Neovim is a
separate process with its own working directory, unrelated to either
`tour`'s cwd or the tour file's directory, so a relative path handed to
`:edit` would be resolved against *Neovim's* cwd, not `tour`'s. This is
purely an internal detail of the RPC call: it does not change what
`tour parse` or anything else displays to a user (per
[design/PARSE.md](../design/PARSE.md), output always shows the slide's
location exactly as authored). Resolution to absolute happens entirely
on `tour`'s side and needs nothing from Neovim:

1. `tour`'s own cwd + the `<tour-file>` argument → absolute path to the
   tour file itself (`filepath.Abs`, using `tour`'s own process
   environment).
2. That absolute tour-file path's directory + the slide's `File` field
   (per [FILE.md](./FILE.md)'s "relative to the tour file's own
   location" rule) → absolute path to the referenced file.

This is the same absolute path used for `ValidateLocations`'s
`os.Stat` call, so the file that gets validated and the file that gets
opened in Neovim are guaranteed to be the same path, computed once.

## Locating Neovim

Implements the lookup from design's "Locating Neovim":

1. `os.Getenv("NVIM")`.
2. If `$HERDR_WORKSPACE_ID` is set, shell out to `herdr workspace get
   <id>`, read `result.workspace.label` from its JSON, and use
   `/tmp/nvim-<label>.sock` if `os.Stat` finds it. Any failure (no
   herdr, bad JSON, empty label, missing socket) falls through.
   Considered: deriving the label from `$HERDR_*` env vars alone — not
   possible, only the workspace id is exported, and labels are
   renameable, so it has to be asked for at startup.
3. If still empty, shell out: `tmux show-environment NVIM` via `os/exec`,
   parsing its `NVIM=<value>` stdout line. A non-zero exit (no tmux, no
   session, variable unset) is treated as "not found," not an error —
   falling through to the next step is expected behavior, not a failure
   mode.
4. `--nvim` flag value, if given.

This is implemented as plain sequential Go, not a strategy/lookup-chain
abstraction — a few fixed steps checked in order don't need a pluggable
interface. (Design originally scoped this to three sources; Herdr was
added later as a fourth, which was a one-function change.)

## Bubble Tea model

```go
type Model struct {
    tourName string
    tourDir  string
    slides   []tourfile.Slide
    current  int // 0-based

    locErrs      map[int]error // keyed by Slide.Index
    nvim         NvimClient    // nil if no session found/connected
    noSessionErr error         // why nvim is nil, e.g. ErrNoNvimSession
    nvimErr      error         // what the status line shows right now

    viewport viewport.Model
    width, height int

    pendingG      bool   // mid g-sequence (gg or g<digits>Enter)
    pendingDigits string
}
```

- **`nvimErr` vs. `noSessionErr`**: these are separate fields because
  what the status line should show for the *current* slide isn't always
  the same thing. `noSessionErr` is fixed at construction (the reason
  there's no Neovim at all, if there isn't one); `nvimErr` is
  recomputed on every `gotoSlide` — from `locErrs[slide.Index]` if the
  current slide's own location is known-bad, from `noSessionErr` if
  there's no Neovim, or `nil`/a jump result otherwise. Collapsing these
  into one field was tried first and had a real bug: leaving a slide
  with a location error for a slide without one, with no Neovim
  connected, left the *previous* slide's error message on screen,
  because nothing overwrote it. `gotoSlide` always sets `nvimErr`
  definitively for the new slide instead of only sometimes touching it.
- **The `g`-prefix key sequence** (`gg` for first, `g<digits>Enter` for
  a specific slide) is the one piece of navigation state that spans more
  than one keypress. `pendingG`/`pendingDigits` implement it as a small
  inline state machine in `handleKey`, not a separate type — it's two
  fields and four cases, entered on `g` and always resolved or
  cancelled by the very next keypress, so a state machine abstraction
  would be more ceremony than the actual logic.
- **`Init` cannot mutate the model Bubble Tea will actually run** —
  `Init() Cmd` returns only a `Cmd`, not an updated `Model`, so any
  field changes made inside it are discarded. `New` calls
  `m.gotoSlide(0)` itself (to set `current`/`nvimErr` for slide 0
  before the first render) and discards *that* call's `Cmd`; `Init`
  then calls `gotoSlide(0)` again on a throwaway copy purely to obtain
  the equivalent `Cmd` to hand back to Bubble Tea. `gotoSlide` is a
  pure function of the model's own fields, so computing it twice is
  cheap and avoids duplicating its branching logic across `New` and
  `Init` separately.
- Jumping to Neovim is done as a `tea.Cmd` (Bubble Tea's mechanism for
  side effects), not inline in `Update`, so a slow/hung RPC call doesn't
  block the TUI from redrawing; its result comes back as a `jumpResultMsg`
  that sets `nvimErr`.
- `View` renders three lines — title bar, the padded body, status bar —
  joined with `\n`, and sets `tea.View.AltScreen = true`. Design's
  mockup shows a bordered box with the title embedded in the top border
  line itself; that specific chrome (drawing a border whose top edge
  also carries text) needs more custom rendering than `lipgloss.Border()`
  gives for free, and isn't exercised by any `ft` scenario, so it's
  deliberately deferred as polish rather than built now — noted here
  rather than silently dropped.
- **Alt screen** (`v.AltScreen = true` in `View`) — without it, every
  frame the TUI ever drew is left behind in the terminal's scrollback
  after quitting, since the renderer just writes and overwrites lines in
  the normal buffer. The alt screen buffer is a separate surface that
  the terminal restores *from* on exit, so quitting leaves the terminal
  exactly as it looked before `tour` launched — confirmed with a live
  smoke test (start the TUI in a scratch tmux pane, quit, `capture-pane`
  shows only what was in the pane before launch).
- **Bars and body margin**: `titleBarStyle`/`statusBarStyle` (`lipgloss.NewStyle()`
  with a `Background` color each, distinct from one another) are applied
  to the *already-justified* bar string — `justify` still does the
  left/right layout math, the style just colors the resulting
  already-full-width line, rather than the style doing width/alignment
  itself. The body gets `bodyMarginX`/`bodyMarginY` on every side: the
  `viewport`'s own width/height are set to the terminal size *minus*
  that margin (`bodyWidth`/`bodyHeight`), and `bodyStyle.Padding(...)`
  then adds the margin back around the rendered viewport — the two have
  to agree, or the body's total footprint drifts from the space actually
  reserved for it between the two bars.
- **Title bar now shows the current slide's code location, not the tour
  filename** (`titleBar`/`currentLocationText`), with the tour's own
  filename moved to the status bar (`statusBar`) — per design's "What
  the user sees" section. `currentLocationText` uses
  `filepath.Base(slide.File)`, not the relative path `tour parse` shows
  (per [design/PARSE.md](../design/PARSE.md)): the title bar is space-
  constrained chrome a reviewer glances at repeatedly, not a report
  read once, so the basename is the better fit there specifically —
  this is a narrower, bar-specific exception to FILE.md/PARSE.md's
  general "show it exactly as authored" rule, not a change to that rule.

## Testing

- **Markdown rendering**: see the "Testing" subsection under "Markdown
  rendering" above.
- **Neovim discovery** (`viewer/locate_test.go`): the process-env-wins
  case is tested directly and unconditionally (`t.Setenv("NVIM", …)`
  always wins regardless of ambient state). The flag-wins and
  nothing-found cases are skipped, with a stated reason, whenever the
  test process's own environment already has tmux `$NVIM` set (Herdr is
  isolated by clearing `$HERDR_WORKSPACE_ID` instead) — which
  is true in this project's own dev session (launched via `nv`), so
  asserting "nothing was found" unconditionally would make the test's
  result depend on the environment it happens to run in rather than on
  the code. No interface was introduced to mock the `tmux` shell-out
  purely to avoid this — three fixed lookup steps didn't seem to
  warrant one, per this doc's original reasoning in "Locating Neovim."
  Herdr's JSON parsing and socket-path formation are tested as pure
  helpers (`parseHerdrWorkspaceLabel`, `herdrNvimSocketPath`).
- **Bubble Tea model** (`viewer/model_test.go`): `Update` is tested
  directly as a pure function against key messages built with
  `tea.KeyPressMsg{Code: r}`, asserting resulting `current`/scroll
  state via a `fakeNvim` (records `Jump` calls instead of dialing a
  real one) and a `plain()` helper that strips ANSI codes before
  substring assertions (glamour styles each word of wrapped narration
  with its own escape sequence, so raw `View().Content` doesn't contain
  a phrase as one contiguous byte run even though it reads as one on
  screen).
- **Neovim RPC integration** (`viewer/nvim_integration_test.go`): gated
  behind `//go:build nvim_integration` so the default `go test ./...`
  doesn't require the `nvim` binary. Spawns a real `nvim --headless
  --listen <tmp socket>`, dials it with the same `ConnectNvim`/`Jump`
  path the viewer itself uses, and verifies against the *real* Neovim
  (a second, independent `nvim.Dial` connection) that the buffer and
  cursor actually moved, and that jumping twice to the same file reuses
  one buffer rather than opening a second. Run explicitly with `go test
  -tags nvim_integration ./viewer/...`.

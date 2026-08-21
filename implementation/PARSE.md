# Parse Command

Implements [design/PARSE.md](../design/PARSE.md).

## Reuse, not reimplementation

`tour parse` calls the same `tourfile.Parse` and `tourfile.ValidateLocations`
functions the viewer calls at open time (per
[implementation/FILE.md](./FILE.md) and [implementation/VIEWER.md](./VIEWER.md)'s
Validation section) — there's no separate validation path to keep in
sync. The command's own code is limited to formatting `[]tourfile.Slide`,
`[]tourfile.ParseError`, and `[]tourfile.LocationError` for a terminal,
and choosing an exit code.

## Output formatting

Options considered:

1. **Plain text, human-readable** — a line per slide
   (`file:line  preview...`), followed by any errors.
2. **Structured output** (`--json` flag, or JSON-only) — machine-parseable
   output for scripting.

**Decision: plain text only, for now.** Matches the project's existing
bias toward plain, tool-free text ([FILE.md](../design/FILE.md)'s own
"why this shape"), and nothing in the design calls for scripting against
`tour parse`'s output — CI/pre-commit usage (per design's "Exit status"
section) only needs the exit code, not parseable output. A `--json` flag
is a straightforward, backward-compatible addition later if a real
scripting need shows up; adding it now would be speculative.

## Narration preview

Truncating a slide's narration to one line with `...` needs two things
the naive approach gets wrong:

- **First line only, not first N characters** — narration is
  multi-paragraph free text (per [FILE.md](../design/FILE.md)); cutting
  by character count could stop mid-sentence one line in even when a
  natural line break exists right there. Take everything up to the
  first `\n` (or the whole text, if it's one line), and truncate *that*.
- **Truncate by rune, not by byte** — narration is arbitrary UTF-8
  prose; slicing a Go `string` by byte count can cut a multi-byte rune
  in half. Truncation converts to `[]rune` first, cuts at a fixed rune
  count (e.g. 60), and appends `...` only if the original was longer
  than that count (a naturally short first line isn't padded with a
  trailing ellipsis it doesn't need).

```go
func preview(text string, maxRunes int) string
```

Pure function, independent of terminal width — the design doesn't
require wrapping the preview to the pane, just keeping the listing
scannable, so a fixed rune budget is enough.

## Error output

Two decisions from [design/PARSE.md](../design/PARSE.md)'s Output
section need concrete handling: the per-slide listing is suppressed
entirely when there's any error, and errors print in color on a
terminal.

- **Short, one-line error messages** — `tourfile.ParseError.Msg` and
  `tourfile.LocationError`'s wrapped `Err` are kept to a plain phrase
  (`"missing @ file:line"`, `"file not found"`,
  `"only 3 line(s), wanted line 99"`), not a restatement of the
  offending source line. An earlier version of `ParseError.Msg`
  included the malformed line's full text via `%q` — that's redundant
  with `tour parse` printing the tour file's own line number right next
  to it (an author can just look at that line), and works against the
  "fit on one line" requirement for narration containing anything long.
- **Color via `lipgloss`, not a hand-rolled ANSI/`x/term` check** — a
  `lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))`
  applied to each error line. `lipgloss` is already a dependency for
  the viewer (per [implementation/VIEWER.md](./VIEWER.md)), and its
  default renderer already does the "is this actually a terminal"
  detection this needs — TTY check, `NO_COLOR`, `TERM=dumb` — via
  `termenv` under the hood, so a plain-text fallback when stdout is
  piped or captured (a file, CI, a test) comes for free rather than
  being hand-implemented a second time alongside the viewer's own color
  handling.
- **Suppressing the slide listing** is a branch on `len(parseErrs) +
  len(locErrs) > 0` before printing anything, not a filter applied
  after building a listing — the two output shapes (design's "Valid"
  and "Invalid" cases) are constructed as alternatives, not one
  superset with parts hidden.
- **Consistent slide numbering, including broken blocks** —
  `tourfile.Slide` carries an `Index` (1-based position among *all*
  blocks in the file, set once, in `Parse`, as each block is flushed —
  not derived later from a slice position). `tourfile.ParseError`
  carries the matching `Slide` number for the block it came from, and
  `tourfile.LocationError.Slide` is the full `Slide`, so its `Index` is
  already correct. This replaced an earlier version where
  `LocationError`'s slide number was the loop position in
  `ValidateLocations`'s `range slides` — that undercounts whenever an
  earlier block failed to parse and was excluded from the `[]Slide`
  slice, so "slide 3" could silently mean the 3rd *successfully-parsed*
  slide rather than the 3rd block in the file. The reported slide count
  (`len(slides) + len(parseErrs)`) uses the same principle: a block that
  fails to parse still occupies a slide position, so it's still counted.
- **Errors ordered by slide, not grouped by type** — `mergeErrors`
  combines `parseErrs` and `locErrs` into one slice of `(slide, text)`
  pairs and sorts by slide number (`sort.SliceStable`, so errors that
  belong to the same slide — which `ValidateLocations` can't currently
  produce more than one of per slide, but nothing enforces that
  invariant at this call site — keep their original relative order
  rather than an arbitrary one). Printing `parseErrs` then `locErrs` as
  two separate loops was the earlier approach; it grouped errors by
  *kind* instead of by *position in the tour*, so a structural error on
  slide 5 could print above a location error on slide 2 — technically
  correct information, presented out of the order an author reading the
  tour top-to-bottom would expect.

## Command wiring

`cmd/parse.go`, a cobra command taking one positional arg
(`cobra.ExactArgs(1)`). `SilenceUsage` is set on this command: a parse
failure is a problem with the *tour file*, not with how `tour parse` was
invoked, so cobra's default "print usage on any RunE error" behavior
would be misleading noise above the actual error list. `SilenceErrors`
stops cobra's own internal error line, but `Execute()`'s returned error
still reaches `main.go` — cobra silencing itself doesn't silence the
caller.

That's what `cmd.ErrReported` (defined in `cmd/root.go`, shared across
all commands) is for: once `runParse` has printed its own report, it
returns `ErrReported` — a bare sentinel with no message of its own,
since nothing downstream needs the text back, only the fact that this
run failed. `main.go` checks `errors.Is(err, cmd.ErrReported)`: if true,
it exits non-zero silently, since the user already saw what's wrong on
stdout; if false — as with `runParse`'s own `os.Open` failure, which
returns the raw error because nothing was printed for that path — it's
an error no command reported itself, so `main.go`'s job is to print
exactly that one. Every `RunE` still only ever returns an `error`
(never calls `os.Exit` itself); `main.go` remains the single place that
turns a failure into a process exit code, which is what keeps `RunE`
testable against its returned error rather than an actual process exit.

## Success formatting

`formatSuccess(renderer *lipgloss.Renderer, slides []tourfile.Slide)
string` is a pure function taking the renderer as a parameter, rather
than `runParse` doing the styling and printing inline as it does for
errors. Column width is computed as the widest `file:line` in the tour
(rune count, via `unicode/utf8`, for the same reason `preview` truncates
by rune rather than byte — narration and paths are arbitrary UTF-8),
and every location is padded to it before the two-space separator and
preview.

## Testing

- **`preview`**: table-driven — multi-line text, text shorter than the
  rune budget (no `...` appended), text exactly at the budget, and
  multi-byte UTF-8 text straddling the truncation point.
- **End-to-end**: fixture `.tour` files (valid, structurally broken,
  location-broken) run through the command with output captured, and
  the exit code plus printed report checked against fixed expectations
  — the same fixture-and-`t.TempDir()` approach used for
  `ValidateLocations` in [implementation/FILE.md](./FILE.md), reused
  rather than building a second fixture set.
- **Color output** — `lipgloss.NewRenderer` derives its color decision
  from whether its `io.Writer` looks like a real terminal, which a
  `bytes.Buffer` never does, so end-to-end tests (which capture into a
  buffer) can only ever exercise the "no color" path. That's exactly
  what covers the "no color when not a terminal" requirement, but it
  can't exercise "color *is* applied on a terminal" — for that,
  `formatSuccess` is called directly with a renderer whose profile is
  forced via `Renderer.SetColorProfile(termenv.ANSI)` (documented by
  lipgloss as existing for this purpose), independent of any real TTY.
  One subtlety: `termenv.WithProfile(...)` passed to `NewRenderer`'s
  constructor does *not* force the profile — `Renderer.ColorProfile()`
  calls `termenv.Output.EnvColorProfile()`, which re-detects from the
  environment regardless of that constructor option.
  `SetColorProfile` is the one path that actually sets
  `explicitColorProfile` and bypasses detection.

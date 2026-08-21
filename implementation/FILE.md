# Tour File Parsing

Implements the format specified in [design/FILE.md](../design/FILE.md).

## Options considered for parsing

1. **Hand-written line scanner** (`bufio.Scanner` over the file,
   splitting on lines that are exactly `---`) — the grammar is: a
   sequence of blocks separated by a literal delimiter line, each block's
   first non-blank line matched against a fixed prefix (`@ `), the rest
   captured verbatim.
2. **Parser-combinator / grammar library** (e.g. `participle`) — declare
   the grammar and get a parser generated.
3. **Whole-file regexp** — read the entire file, split on a `---`
   pattern, apply a second regexp per slide to pull out the location
   line.

**Decision: hand-written line scanner.** The format was deliberately
kept trivial in the design (per FILE.md's "why this shape") specifically
so it wouldn't need real parsing machinery — a grammar library would be
solving a harder problem than the one that exists. A line scanner also
gives line numbers for free, which the Validation design (per
[VIEWER.md](../design/VIEWER.md)) needs to report structural errors
usefully ("line 14: missing `@ file:line`"). A whole-file regexp can do
this format too, but loses per-line position tracking and is harder to
extend if the grammar ever grows a third element.

## Structure

```go
type Slide struct {
    File string
    Line int
    Text string
}

type ParseError struct {
    Line int
    Msg  string
}

func Parse(r io.Reader) ([]Slide, []ParseError)
```

`Parse` never stops at the first error — it keeps scanning past a
malformed slide (treating the next `---` as the recovery point) so every
structural problem in the file is collected in one pass. This is what
[VIEWER.md](../design/VIEWER.md)'s Validation section requires: all
structural errors reported together, not one at a time as the author
fixes and reruns.

## `file:line` parsing

Matched with `^@ ([^:]+)(?::(\d+))?$` rather than
`strings.LastIndex(line, ":")`: a regexp makes "the trailing segment, if
present, must be all digits" part of the match itself, so a path that
happens to contain a colon but no trailing line number is correctly
rejected as malformed rather than silently misparsed. The `:line` group
is optional per design's decision to allow a bare `@ file` — when
absent, `Slide.Line` is set to `1` at parse time, so nothing downstream
(validation, the jump command) needs to know the line was ever omitted;
`@ file` and `@ file:1` produce an identical `Slide`.

## Location validation

Separate from structural parsing — this step needs the filesystem, so it
runs after `Parse` succeeds, over the already-parsed `[]Slide`:

```go
func ValidateLocations(tourDir string, slides []Slide) []LocationError
```

For each slide: resolve `File` relative to `tourDir` (the tour file's
own directory, per FILE.md), `os.Stat` it, and if it exists, count lines
with a `bufio.Scanner` up to `Line` (stopping early once the count is
reached — no need to read the rest of a large file). Every slide is
checked regardless of earlier failures, matching the "check every slide
up front" requirement in VIEWER.md's Validation section.

## Testing

- `Parse`: table-driven tests over literal tour-file strings — valid
  multi-slide files, missing location lines, malformed `file:line`,
  slides with no text, trailing/leading blank lines, a narration body
  that itself contains something that looks like `---` mid-paragraph
  (should not happen per the format, documented as an accepted
  ambiguity rather than escaped).
- `ValidateLocations`: uses `t.TempDir()` to build small fixture trees
  (files of known line counts) and asserts which slides report errors.
  No filesystem mocking needed since real temp files are cheap and
  exercise the actual `os.Stat`/scan path.

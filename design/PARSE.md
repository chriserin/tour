# Parse Command

## Problem

Right now, a `.tour` file only gets checked when someone opens it in the
viewer, next to a running Neovim (per [VIEWER.md](./VIEWER.md)'s
Validation section). An author drafting a tour — or a CI job checking
one before it's merged — has no way to find out whether the file is
well-formed without launching the TUI and having a Neovim session ready.

## What

A CLI command, `tour parse <tour-file>`, that runs the same checks the
viewer runs at open time and reports the result directly: how many
slides were found, each slide's `file:line`, and any error encountered
— with no TUI and no Neovim involved.

## Why this shape

- **The same checks the viewer uses, not a separate notion of
  "valid"** — this command is a non-interactive front end onto exactly
  [VIEWER.md](./VIEWER.md)'s Structural validity and Location validity
  checks. It doesn't define its own rules; "does `tour parse` accept
  this file" and "will the viewer accept this file" are the same
  question by construction, not two things that can drift apart.
- **No Neovim required** — validating a tour file's own well-formedness
  doesn't depend on a live editing session, so an author can check a
  tour before Neovim is even running, and it can run unattended (CI, a
  pre-commit hook) where there's no Neovim at all.
- **Human-readable summary, not just pass/fail** — an author fixing a
  broken tour needs to see _which_ slide and _what's_ wrong, and seeing
  the slide count and each resolved `file:line` even when there are no
  errors is a useful sanity check on its own (did the tour end up with
  the number of slides I meant to write, in the order I meant?).

## Output

Output has two distinct shapes, depending on whether the tour is valid —
not one shape that grows an error section tacked on:

**Valid** (no structural or location errors): the number of slides
found, then each slide's `file:line`, in tour order, alongside a
truncated preview of its narration text (e.g. `lib/app/auth.ex:42  This
code does x...`) — a bare `file:line` list doesn't say what a slide is
_about_, so confirming "is this actually the order I meant to tell this
story in" needs a glimpse of the narration, not just the location.
Truncated to one line, with `...` if the narration continues past it, so
the listing stays scannable for a tour with many slides. The `file:line`
shown is exactly what the author wrote in the slide's location line (per
[FILE.md](./FILE.md), relative to the tour file) — never resolved to an
absolute path. An author reading this output wants to recognize what
they typed; an absolute path is an internal detail needed only for the
filesystem checks themselves, not something useful to show back.

Each slide's `file:line` is padded to the width of the longest
`file:line` in the tour, so every narration preview starts in the same
column — a ragged list where each preview starts wherever its own
`file:line` happened to end is legible for three slides and a chore to
scan for thirty. The `file:line` column is printed in a distinct color
from the narration preview (per this section's Coloring rule below), so
even without reading closely, location and narration read as two
different things rather than one run-on line.

**Invalid** (one or more errors): the slide count and error count as a
one-line summary (e.g. `8 slides, 1 error`), then every error — every
structural error (per VIEWER.md's Validation section: all of them, not
just the first) and every location error (file doesn't exist, line out
of range) — ordered by the slide each belongs to, structural and
location errors interleaved rather than grouped by type, so the list
reads top to bottom in the same order the slides appear in the tour.
The per-slide listing is _not_ shown in this case. An author
fixing a broken tour wants to find what's wrong, and burying one error
among a dozen lines of otherwise-fine slides makes it harder to spot,
not easier — the two shapes exist because "here's what's broken" and
"here's a sanity-check listing of a tour that's fine" are different
jobs, not the same report at two sizes.

A block that fails to parse still counts as a slide — its position in
the tour is unambiguous even though its content isn't well-formed, so
excluding it from the slide count would undercount the tour and make
"slide 3" mean different things depending on whether an earlier slide
happened to be broken. Every error, structural or location, is
therefore anchored the same way: by the slide number it belongs to
(1-based, counting every block in the file in order). A structural
error additionally names the tour file's own line number, since that's
what an author needs to jump straight to the broken block in an editor
— a location error doesn't need this, since its slide's `file:line` is
already known and shown.

Each error is one line, kept short — a slide/line reference and a brief
reason, not a restatement of the offending text. Errors are printed in
color (bold red) when standard output is a terminal, so a broken tour
is visually unmistakable at a glance; output is plain text when stdout
isn't a terminal (piped to a file, captured by CI), so redirected output
and logs don't fill with escape codes.

## Coloring

The same terminal-only rule applies to the valid case as to errors:
color is used when standard output is a terminal, and omitted (plain
text) when it isn't — a piped or captured `tour parse` should never
emit escape codes for a script or log file to trip over.

On a terminal, a successful listing uses two colors, chosen so the
report reads correctly even to someone not consciously parsing the
colors — each one reinforces something the plain-text layout already
says, rather than being the only place that information lives:

- **The slide count line, when there are no errors, is bold green** —
  a successful `tour parse` should be recognizable from a glance at the
  first line alone, the same way a broken one is (bold red, per Output
  above) — a colorless success next to a colorful failure would read as
  "this command doesn't have an opinion" precisely when it does.
- **Each slide's `file:line` column is a distinct, muted color from the
  narration preview** — not for decoration, but so the eye can track a
  single column down a list of many slides the way column alignment
  alone doesn't fully achieve when locations vary in length. The
  narration preview itself stays the terminal's default color, since
  it's prose, not structured data — coloring it would suggest it means
  something beyond what it says.

## Exit status

Non-zero if any structural or location error was found, zero otherwise
— so the command composes directly with CI and pre-commit hooks without
needing to parse its own output to decide pass/fail.

## Non-goals (for now)

- Opening the viewer or connecting to Neovim — this command never does
  either, by design (see "why this shape" above).
- Fixing or reformatting a tour file — this command only reports.

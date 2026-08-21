# Tour File

## Problem

The tour CLI needs something to read: a single artifact that holds an
entire tour so it can be authored once (by hand or by AI) and played back
later, shared, reviewed, and diffed like any other text.

## What

A tour is stored as one plain text file. The file is an ordered list of
slides. Each slide has:

- **text** — the narration for that step, free-form prose.
- **location** — a `file:line` pointing at the code the slide refers to.

The file's top-to-bottom order *is* the tour's order — there is no
separate ordering field, index, or manifest. Reordering slides means
reordering lines in the file.

## Why this shape

- **One file, not a directory of comments** — the whole narrative lives in
  a single place, so it can be authored, reviewed, and versioned as one
  unit, the same way a commit message or a design doc is.
- **Plain text** — readable and writable without tooling, easy to diff in
  a PR, trivial for an AI to generate from a diff and for a human to
  hand-edit afterward. No binary or app-specific format.
- **Order encoded by position** — matches the vision's requirement that
  the tour builds a deterministic narrative; keeping order implicit in
  file position (rather than explicit slide numbers) avoids renumbering
  when slides are inserted or removed.
- **`file:line`, not a line range or diff hunk** — a single point is
  enough to direct the editor's attention; the narration text is what
  explains what to look at around that point.

## Elements

A tour file is a sequence of slides separated by a line containing only
`---`. This mirrors a convention already familiar from Markdown
frontmatter and horizontal rules, so it reads as "just text" rather than
a bespoke syntax, and it's unambiguous as long as a slide's narration
never needs a bare `---` line of its own.

```
@ path/to/file.rb:42
This is the narration for the first slide. It can span
multiple lines and multiple paragraphs.
---
@ path/to/other_file.rb:7
Narration for the second slide.
---
@ path/to/file.rb:108
Back to the first file, later on — order is whatever
order the slides appear in, not file order.
```

Each slide has exactly two elements, in a fixed order:

- **Location line** — the *first* non-blank line of the slide, starting
  with `@ ` followed by a `file:line`. Putting it first, and marking it
  distinctly, means both a human scanning the file and a simple line-based
  parser can find "where does this slide point" without reading the prose.
- **Text** — everything after the location line, up to the next `---` (or
  end of file). Free-form; blank lines and multiple paragraphs are fine
  since the boundary is the separator, not a line count.

### `file:line`

- **file** — a path relative to the tour file's own location (not the
  current working directory), so a tour stays correct when run from
  anywhere. Forward slashes only, matching how the path would appear in a
  diff.
- **line** — a 1-indexed line number, matching both Neovim's and diff
  tools' line numbering, so a number copied from a diff hunk or `git
  blame` can be pasted in as-is. **Optional** — `@ path/to/file.rb` (no
  `:line`) is valid and means line 1. This is for slides about a whole
  file rather than one spot in it (a new file being introduced, say),
  where writing `:1` would be a line number with no narrative meaning,
  just padding to satisfy the syntax.
- Only a single line is addressed, not a range — consistent with the
  file-level decision that a slide points at one place and lets the
  narration explain the surrounding context. Omitting the line entirely
  is the one exception to "always specify a location precisely," and is
  narrow enough not to be ambiguous: `@ file` and `@ file:1` always mean
  the same thing, so there's no case where a reader has to guess which
  was intended.

## Naming convention

Tour files use a `.tour` extension. This gives them a distinct identity
in a directory listing, in `git status`, and in diffs — a `.md` full of
prose reads as documentation-in-progress, while `.tour` reads
unambiguously as a playable artifact. It also lets the viewer (and
editor tooling, later) find tour files by extension rather than by
sniffing content.

## Non-goals (for now)

- A binary or structured-data format (e.g. JSON/YAML) — plain text is the
  requirement, not just a default.
- Embedding code snippets or diffs inside the file — the file points at
  the code, it doesn't duplicate it.
- Multiple tours per file.

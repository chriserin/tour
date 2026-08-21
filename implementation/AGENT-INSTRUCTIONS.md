# Agent Instructions

Implements [design/AGENT-INSTRUCTIONS.md](../design/AGENT-INSTRUCTIONS.md).

## Options considered for storing the instructions text

1. **Go string literal** — the instructions text lives inline in a `.go`
   file as a raw string constant.
2. **`go:embed` a separate text file** — the prose lives in its own
   file (e.g. `agentinstructions/instructions.md`), embedded into the
   binary at compile time via `//go:embed`.

**Decision: `go:embed`.** The instructions are prose describing a format,
not code — keeping it in its own Markdown file means it can be edited,
diffed, and reviewed like the design docs it's derived from, without
Go-string escaping getting in the way, while still compiling into a
single self-contained binary with no runtime file dependency. This also
directly serves design's "one source of truth" requirement: the embedded
file's content can be checked against `design/FILE.md` in review, or
generated from it, rather than drifting as two independent hand-written
prose blocks.

## Command

`tour agent-instructions` prints the embedded file's contents to stdout
verbatim and exits 0. No flags, no templating — the design explicitly
scopes this to "print the format," not an interactive or parameterized
command.

## Testing

A single test asserts the command's stdout equals the embedded file's
contents exactly (i.e. that the command isn't accidentally
transforming, truncating, or adding to it). The content's *accuracy*
against `design/FILE.md` isn't something a test can check — that's a
review-time concern each time `FILE.md` changes, not a runtime one.

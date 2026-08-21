# Agent Instructions

## Problem

[FILE.md](./FILE.md)'s format was deliberately kept plain and minimal so
that an AI could generate a tour, but that's only true if the generating
agent actually knows the format.

## What

A CLI command, `tour agent-instructions`, that prints the tour file
format to stdout: the `.tour` naming convention, the slide separator,
the location line syntax, and the `file:line` rules. An agent authoring
a tour runs this first (or has its output placed in context some other
way) and then has everything it needs to emit a valid `.tour` file,
without needing any other tour-specific knowledge.

## Why this shape

- **A command, not a doc pointer** — an agent operating in a repo can
  run a command and read its output; it can't be assumed to have
  filesystem access to `design/FILE.md`, or to have been told to go
  read it. Making the spec fetchable at runtime means correctness
  doesn't depend on what happened to be in the agent's prior context.
- **Printed instructions, not a schema file** — plain stdout text is the
  same shape as the format it describes: readable without tooling, and
  easy to drop straight into a prompt or an agent's context.
- **One source of truth** — the instructions describe the same format
  as [FILE.md](./FILE.md); they aren't a separately hand-maintained
  description that can drift out of sync as the format evolves.

## What the instructions contain

- The `.tour` naming convention.
- The slide separator (`---`).
- The location line syntax (`@ file:line`, must be the first non-blank
  line of a slide).
- The `file:line` rules: path relative to the tour file, forward
  slashes, 1-indexed line number.
- A minimal worked example (a two-slide tour) — an example is often more
  directly usable by a generating agent than the prose rules alone.

## Non-goals (for now)

- Guidance on _what_ narrative to write — how to summarize a diff, how
  many slides to produce, tone. This command teaches syntax, not
  editorial judgment; constructing the narrative is the agent's job.
- Validating a tour file — that's a viewer/CLI concern (see
  [VIEWER.md](./VIEWER.md)'s Validation section), separate from handing
  out instructions.
- Any authoring or editing features inside the viewer itself.

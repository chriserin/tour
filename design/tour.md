# Tour

## Problem

Reviewing code changes is hard when a diff or feature spans many files. A
raw diff has no narrative — it doesn't tell you what order to read things
in, or why a given hunk matters. Reviewers have to reconstruct the story
themselves.

## What

A TUI CLI app that plays back a guided "tour" of a code change: a linear
sequence of slides, each pairing some explanatory text with a location in
the code (`file:line`). Stepping through the tour drives Neovim (via its
remote/RPC capability) to jump to the corresponding location, so the
reviewer reads narration in the TUI while watching the relevant code move
into view in their editor.

## Why this shape

- **Linear ordering** — a tour is a sequence, not a graph or a set of
  comments. The author chooses the order deliberately to build a narrative,
  the same way a slide deck does. This is what distinguishes a tour from
  ordinary PR comments, which are anchored to the diff's order, not a
  story's order.
- **Plain text slides** — slides must be trivial to read, write, and diff.
  This also makes them easy for an AI to generate from a diff or feature
  branch, and easy for a human to hand-edit afterward.
- **Neovim integration** — the reviewer's attention should stay on the
  actual code, not a duplicated view of it inside the tour tool. The TUI
  supplies narration and controls pacing; Neovim supplies the code.

## Core concepts

- **Slide** — one step of a tour: text plus a `file:line` target.
- **Tour** — an ordered list of slides describing one diff or feature.
- **Author** — a human or an AI producing a tour, typically from a diff.
- **Viewer** — the person stepping through the tour to understand a change.

## Core interaction

1. A tour is authored (by hand or generated) as a plain text file.
2. The viewer opens the tour in the TUI, with Neovim already running.
3. The TUI shows the current slide's text; advancing the slide sends the
   slide's `file:line` to Neovim, moving the editor's view there.
4. The viewer moves forward/backward through the tour at their own pace.

## Non-goals (for now)

- Branching or non-linear tours.
- Editors other than Neovim.
- Live re-authoring of a tour while viewing it.

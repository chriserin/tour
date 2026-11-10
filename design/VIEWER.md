# Slide Viewer

## Problem

A tour file ([FILE.md](./FILE.md)) is just narration and locations sitting
in a text file. Something has to play it back: show the viewer where they
are in the narrative, and drive Neovim to the right place, one slide at a
time.

## What

A TUI that displays the current slide's narration and lets the viewer
step forward and backward through the tour. Each time the current slide
changes, the viewer sends that slide's `file:line` to a running Neovim
instance so the editor follows along.

## Why this shape

- **One slide on screen at a time** — matches the tour's own model: a
  linear narrative meant to be read in order, at the viewer's pace, not a
  document to skim or scroll through non-linearly.
- **Narration and code stay in separate panes of attention** — the TUI
  owns the story, Neovim owns the code, per [tour.md](./tour.md). The
  viewer doesn't reimplement a code pane; it only ever shows text and
  position.
- **Position always visible** — the viewer always shows where they are
  in the tour (e.g. "3 of 9"), since the whole point of a tour is a
  deterministic sequence, and losing your place in it defeats that.
- **Neovim must already be running** — the viewer targets an existing
  session rather than opening its own, so it fits into whatever editing
  setup the reviewer already has rather than replacing it.

## Core interaction

1. Launch the viewer against a tour file. It loads the slides and shows
   slide 1, and immediately jumps Neovim to slide 1's location.
2. The viewer displays:
   - the current slide's narration text
   - the current position in the tour (e.g. "3 of 9")
   - the current slide's `file:line`, for confirmation
3. Advancing to the next or previous slide updates all three, and sends
   the new location to Neovim.
4. Jumping directly to an arbitrary slide (not just next/prev) is
   supported, since a reviewer may want to jump back to slide 1 from
   slide 9 without stepping through everything in between.
5. Quitting the viewer leaves Neovim as-is — it doesn't close or reset
   the editor.

## Launching

The viewer must not assume it's a child of the Neovim it's driving — a
tmux pane running `tour` alongside a Neovim pane is just as valid a setup
as launching from Neovim's own `:terminal`, and the two shouldn't need
different commands or flags to work.

- **Command**: `tour <tour-file>` — a single positional argument, the
  path to the tour file. No project-wide config or discovery step; a
  tour is fully described by that one file.
- **On launch**: locate Neovim (see below), load and parse the tour
  file, jump to slide 1, and render the screen. If the connection fails,
  the viewer still renders slide 1's narration (per the "no Neovim
  session found" failure mode) rather than exiting.

## Locating Neovim

The viewer needs a socket/address to talk to an already-running Neovim
(per this doc's "must already be running" decision). It's found by
checking, in order:

1. **`$NVIM` in the process environment** — set automatically by
   Neovim's own `:terminal` for its direct children. This is the most
   specific signal available: it names the exact Neovim instance the
   viewer is literally running inside of, so it always wins when
   present.
2. **The Herdr workspace's socket** — inside Herdr (`$HERDR_WORKSPACE_ID`
   set), the viewer asks Herdr for the current workspace's label and
   uses `/tmp/nvim-<label>.sock`, the socket a launcher starts Neovim on
   for that workspace. Skipped if that socket doesn't exist yet. Like
   tmux below, this lets a sibling pane find Neovim without being its
   child.
3. **`$NVIM` in the tmux session environment** — set once, at session
   scope, by a launcher shell function (`nv`) that starts Neovim with
   `nvim --listen /tmp/nvim-$(tmux display-message -p '#S').sock` and
   records that same path with `tmux setenv NVIM ...`. Any other pane in
   the session — including ones opened before or after that Neovim
   started — can ask tmux for it directly via `tmux show-environment
NVIM`, without the viewer needing to re-derive the socket path
   itself. This is what makes a separate tmux pane work without being a
   child of Neovim at all.
4. **`--nvim <address>`, given explicitly** — used if nothing above
   matched, and is the only option outside Herdr and tmux, where there
   is no session to key off.

Steps 1 and 3 share the same variable name deliberately: precedence
falls out of ordinary environment inheritance rather than needing
explicit conflict-resolution logic — a pane's own process env is simply
checked before tmux's session-scoped one.

## Jump target

A jump always targets Neovim's _currently active window_, opening the
slide's file there (as a new buffer if it isn't already open) rather
than in a split or a window the tour tries to designate as "the code
pane." Neovim may have several windows open — the reviewer's own splits,
layout, and window management are theirs to control, and the tour has
no way to know which one they consider "the" code window. Targeting
whichever window is active leaves that decision with the reviewer: the
window they're focused on right when they advance a slide is the one
that follows the tour.

## Navigating

Keys are single-key, vim-flavored, and require no modifier — a reviewer
is expected to be tapping through many slides in a row.

| Key(s)                     | Action                                 |
| -------------------------- | -------------------------------------- |
| `n`, `space`, `l`, `→`     | next slide                             |
| `p`, `h`, `←`              | previous slide                         |
| `g` then a number, `Enter` | jump to that slide number (e.g. `g3⏎`) |
| `gg`                       | jump to first slide                    |
| `G`                        | jump to last slide                     |
| `j`, `↓`                   | scroll body down                       |
| `k`, `↑`                   | scroll body up                         |
| `q`                        | quit the viewer                        |

Stepping past the last slide with "next" or before the first slide with
"previous" is a no-op (the viewer stays put) rather than wrapping around
— wrapping would silently undo the "always know where you are" property
the position indicator exists to provide.

`h`/`l` move along the "which slide" axis and `j`/`k` move along the
"where in this slide's text" axis, keeping the two vim-flavored pairs
doing distinct jobs rather than overloading either one. Scroll position
resets to the top of the body on every slide change (next, previous, or
direct jump) — otherwise a reviewer could land on a new slide already
scrolled partway down, left over from wherever the previous slide's body
happened to be scrolled to.

## What the user sees

```
 auth.ex:42                                                slide 3 of 9

    The old session check happened inline in the request
    handler. Pulling it into `authenticate/1` means the same
    check can be reused by the websocket handler added in the
    next slide.

 auth-refactor.tour                              n/space next  p prev  q quit
```

- **Title bar** — the current slide's code location on the left (just
  the file's basename, not its full path — the title bar is prime real
  estate, and a directory prefix is rarely what a reviewer needs at a
  glance), `slide X of Y` position on the right. Position is the one
  piece of chrome that's always present, since losing track of it is the
  specific failure the viewer exists to prevent. When the current
  slide's jump is known to fail, the location is replaced by the error
  instead — this is the bar a reviewer's eye is already on, so a broken
  jump surfaces exactly where they'd otherwise expect to see where they
  landed.
- **Body** — the current slide's narration, left-aligned, wrapped to the
  pane width, with margin on every side so it doesn't sit flush against
  the bars or the terminal edge. Long narration scrolls within the body
  via `j`/`k` (`↓`/`↑`); it does not spill into or shrink the
  title/status chrome. See "Body" below for how its content is rendered.
- **Status line** — the tour's own filename on the left (secondary
  information — which bar it lives in matters less than the code
  location above), key hints on the right.
- The title and status bars each have their own background color,
  distinct from the body and from each other, so the three regions of
  the screen read as separate chrome/content/chrome bands at a glance
  rather than plain text that happens to be positioned in three groups.

## Body

The body renders Markdown narration in full — headers, lists, tables,
links, code blocks, blockquotes, and inline emphasis/code spans, in
addition to plain prose. An earlier version of this design supported
only a narrow inline subset (bold/italic/code spans), on the reasoning
that a slide is one narration block in a fixed-size body rather than a
document, so block-level elements had nowhere sensible to go. That
reasoning doesn't hold up: a heading renders as bold/underlined text, a
list as indented bullet lines, a table as a compact terminal table —
each has a perfectly ordinary terminal rendering, they just hadn't been
implemented yet. And a narrow subset creates a worse problem than the
one it solves: an author who writes `# Setup` in good faith and sees it
come back as literal `# Setup` has no way to tell "this is deliberately
unsupported" from "the renderer is broken." Supporting the whole
Markdown language removes that ambiguity entirely — there's no
unsupported syntax left to be mistaken for a bug.

Tables may wrap or compress to fit the pane width, since the body's
width is whatever the terminal pane happens to be, not something a
slide author controls when writing one.

## Validation

A tour is meant for review, so a broken slide shouldn't surface only
once a viewer happens to step onto it — the whole tour is checked when
it's opened, before slide 1 is shown, not incrementally during playback.
This is cheap: it costs no more than a single pass over the file plus a
stat and line count per referenced file, all work the viewer would
eventually do anyway.

Two things are checked, for every slide at once:

- **Structural validity** — the file parses per [FILE.md](./FILE.md):
  slides separate cleanly on `---`, and each has a well-formed
  `@ file:line` location line. This is self-contained — the file's own
  structure is the only thing being checked, no filesystem lookups
  needed — so it's trivial to run in full before anything is displayed.
- **Location validity** — each slide's `file:line` resolves: the file
  exists (relative to the tour file, per FILE.md) and has at least
  `line` lines. Checking every slide's location up front, rather than
  only the one about to be shown, is what lets the viewer report a
  location that will break _before_ the reviewer stumbles onto it
  mid-tour.

What happens with the results:

- **No errors** — playback proceeds normally, starting at slide 1.
- **Structural errors** — the tour fails to open. Every structural
  problem found is reported at once, not just the first, so the author
  can fix the file in one pass. Nothing is shown, since a file that
  doesn't parse has no reliable "slide 1" to fall back to.
- **Location errors only** — the tour still opens; a slide with a bad
  location is still a slide, just one whose jump will fail. Affected
  slides are marked in the position indicator, and landing on one shows
  its narration with the location error in place of the code location on
  the title bar — reflecting that a stale reference (file moved, file
  got shorter) is an ordinary side effect of code changing, not a
  reason to block review of the rest of the tour.

## Failure modes to account for

- **No Neovim session found** — the viewer must tell the viewer this
  clearly rather than silently failing to move the cursor; narration can
  still be read without Neovim.

## Non-goals (for now)

- Editing slides from within the viewer.
- Any code display inside the TUI itself.
- Non-linear/branching navigation beyond direct jump-to-slide.
- Multiple simultaneous Neovim targets.

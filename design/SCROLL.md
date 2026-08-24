# Scroll Position

## Problem

A jump (per [VIEWER.md](./VIEWER.md)'s Jump target section) moves the
cursor to a slide's line, but where that line ends up *within the
window* is left to Neovim's own scrolling behavior — typically centered,
or wherever the reviewer's `scrolloff`/autocmds put it. For a slide
pointing at the start of a function, a centered cursor often means half
the function is above the fold and only a few lines of it are visible
below, when the whole point of the slide is to read the function that
follows. The tour has no way to say "show what comes after this line,"
only "put the cursor on this line."

## What

An optional third field on a slide's location: `file:line:scroll`,
where `scroll` is the number of lines of context to leave visible
*above* the target line. Landing on the slide positions the window so
exactly that many lines precede the target line at the top of the
window — overriding whatever centering the reviewer's own Neovim
configuration would otherwise apply, for that slide only.

```
@ lib/app/auth.ex:42:4
The old session check happened inline in the request handler. This
pulls it into authenticate/1 — read down from here to see the whole
function.
```

`scroll` is **optional**, same as `line` — most slides don't care
exactly where the line lands in the window, only that it's on screen,
and forcing an explicit position on every jump would fight the
reviewer's own scrolling setup for no reason. It's there for the slides
where it matters: pointing at the top of a function, a block, or a
section the narration wants read downward from.

## Why this shape

- **An offset, not a keyword like "top"** — `:42:0` pins line 42 to the
  very top of the window (showing the maximum possible below it),
  `:42:4` leaves a few lines of what precedes it visible for context,
  matching how a reviewer would naturally want to see a line "near the
  top, with a little breathing room above." A keyword like `top` only
  covers the `0` case and would need a second mechanism the moment
  someone wants partial context, so the general form is the offset
  itself.
- **Relative to the target line, not an absolute window row** — the
  same slide should scroll the same way regardless of how tall the
  reviewer's Neovim window happens to be. "4 lines of context above"
  means the same thing in a short window and a tall one; "row 4 of the
  window" does not.
- **An empty field defaults, the same whichever field it is** —
  `@ file::4` omits `line` (defaults to `1`, same as `@ file` already
  defaults it today per [FILE.md](./FILE.md)), and `@ file:42:` omits
  `scroll` (defaults to "not specified" — identical to `@ file:42`, no
  override). Neither is treated as malformed. A positive `scroll`
  against a defaulted `line: 1` clamps to showing the window from the
  very top, since there's nothing above line 1 to reveal — that clamp
  isn't a special case invented for this: it's the same "`scroll`
  larger than makes sense for this line" outcome that already has to be
  handled for `@ file:2:4` or any slide near the top of a short file,
  and is already a harmless degradation rather than an error (see Jump
  behavior in [implementation/SCROLL.md](../implementation/SCROLL.md)).
  Rejecting an empty field while accepting the equivalent value spelled
  out (`@ file::4` vs. `@ file:1:4`, or `@ file:42:` vs. `@ file:42`)
  would make the same effective location valid under one spelling and
  malformed under another, for no reason a reviewer could point to. A
  bare trailing colon with nothing after it and no second colon
  (`@ file:`) is unchanged and still malformed — that one isn't "a field
  left empty," it's the existing single-`line` form given no digits at
  all, the same as before this feature.
- **A per-slide override, not a viewer-wide setting** — the need is
  specific to individual slides (a function's start, a block worth
  reading downward from), not a blanket "always scroll to the top"
  policy, which would be wrong just as often as the default centering
  is: plenty of slides point at a line in the *middle* of the relevant
  context, where centering (or leaving Neovim's own behavior alone) is
  exactly right. Putting the control in the tour file, authored
  per-slide, means the tour's author — who knows what each slide is
  about — decides when it's worth overriding, rather than the viewer
  guessing from the code.
- **Omitted means hands off, not a default of `0`** — a slide without
  `:scroll` sends only the cursor move, leaving Neovim's own
  centering/`scrolloff` behavior in charge, the same as before this
  feature existed. Silently defaulting every jump to `scroll: 0` would
  mean every tour, even ones written before this feature, changes
  behavior the moment the viewer that supports it is used — the
  reviewer's own Neovim configuration should still be what plays by
  default when a slide doesn't ask for anything different.

## Non-goals (for now)

- Automatically inferring a good `scroll` value from the code at that
  line (e.g. detecting "this is a function start"). The tour's author
  decides when it matters, the same way they decide what the narration
  says.
- Scroll positioning relative to the *bottom* of the target line's
  region (e.g. "keep this line visible near the bottom"). Every use
  case driving this feature is "show more of what's below," which an
  above-the-line offset already covers.
- Horizontal scroll position. Only vertical placement of the target
  line is in scope.

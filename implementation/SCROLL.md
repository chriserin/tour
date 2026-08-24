# Scroll Position

Implements [design/SCROLL.md](../design/SCROLL.md).

## Grammar

`tourfile.locationRe` (per [implementation/FILE.md](./FILE.md)) grows a
second optional segment, and its existing `line` group relaxes from
`\d+` to `\d*`. Per [design/SCROLL.md](../design/SCROLL.md)'s "an empty
field defaults, the same whichever field it is" decision, `line` and
`scroll` are each independently optional and independently allowed to
be empty:

```
^@ (?P<file>[^:]+)(?::(?P<line>\d*))?(?::(?P<scroll>\d*))?$
```

Named groups (`file`, `line`, `scroll`), not positional ones — Go's
`regexp` doesn't support `(?x)` free-spacing/comment mode (that's a
PCRE feature, not part of RE2's syntax), so a name on each group is the
available way to make `m[...]` reads self-explanatory instead of magic
indices. `parseBlock` looks each one up once via `locationRe.SubexpIndex`
into package-level `locationFile`/`locationLine`/`locationScroll`
constants, rather than calling `SubexpIndex` (a linear name search)
on every parse.

- `file`, unchanged from before this feature.
- `line` — present whenever there's at least one colon after `file`;
  its captured text is either digits or empty.
- `scroll` — present only when there's a second colon; same
  digits-or-empty shape.

`parseBlock` resolves both fields the same way: an empty (or absent)
capture defaults, a non-empty one is parsed with `strconv.Atoi`.
`Line` defaults to `1`, `Scroll` defaults to `nil` (not `0` — see
`Slide.Scroll` below for why that distinction matters). This one rule
covers every shape: `@ file` (both absent), `@ file:` (`line` empty —
previously malformed under the old `\d+`-only grammar; now just
`Line: 1`, the same as omitting it, since a colon followed by nothing
carries no more information than no colon at all), `@ file:42` (`line`
set, `scroll` absent), `@ file:42:4` (both set), `@ file::4` (`line`
empty, `scroll` set), and `@ file:42:` (`line` set, `scroll` empty).

What still doesn't match, and remains malformed: any non-digit content
in a field that's present at all, e.g. `@ file:abc` or `@ file:42:-1`
— `\d*` only ever consumes digits, so leftover non-digit characters
before the line's end mean the whole pattern fails to match (Go's
`regexp` requires the full anchored `^...$` to match, not just a
prefix), falling through to the existing "malformed `@ file:line`"
`ParseError`.

Go's `regexp` doesn't distinguish "group didn't participate in the
match" from "group participated and matched an empty string" — both
read back as `""` from `FindStringSubmatch`. That distinction is never
needed here: every case where a field's capture is `""`, whichever way
it got there, resolves to the same default, so treating them
identically is correct, not a limitation being worked around.

## `Slide.Scroll`

```go
type Slide struct {
    Index  int
    File   string
    Line   int
    Scroll *int
    Text   string
}
```

`*int`, not `int`, because `0` is a meaningful, distinct value here
(`:42:0` — pin the target line to the very top) that must be
distinguishable from "not specified." A sentinel like `-1` would work
too, but reads as a magic number at every call site that touches
`Scroll`; a nil-checked pointer says "absent" directly. `ParseBlock`
sets it to `nil` when group 3 didn't match, and to a pointer at the
parsed `strconv.Atoi` value otherwise — no clamping or validation
beyond what `\d+` already guarantees (non-negative).

`Scroll` is not part of location validation
(`tourfile.ValidateLocations`): unlike `File`/`Line`, it doesn't
reference anything on disk, so there's nothing about it a filesystem
check could find wrong. A `scroll` offset that's larger than the
target file's line count isn't an error — see Jump behavior below.

## Jump behavior

`NvimClient.Jump` gains a third parameter:

```go
Jump(file string, line int, scroll *int) error
```

`jumpLua` — originally a Go string constant in `viewer/nvim.go`, later
moved to its own embedded `viewer/jump.lua` file (`//go:embed`, read
into a `var jumpLua string`) once the script's own explanatory comments
grew past what was comfortable to keep inside a Go string literal — is
extended to act on it after the cursor move, only when a scroll offset
was actually given:

```lua
local file, line, scroll = ...
vim.cmd.edit(vim.fn.fnameescape(file))
vim.api.nvim_win_set_cursor(0, {line, 0})
if scroll ~= vim.NIL then
  vim.wo.scrolloff = 0
  vim.fn.winrestview({ topline = math.max(1, line - scroll) })
else
  vim.wo.scrolloff = -1
end
```

Options considered for the scroll mechanism itself:

1. **`winrestview({topline = ...})`** — sets the window's top visible
   line directly.
2. **`zt` plus `\<C-e>`/`\<C-y>` scrolling**, or **`normal! zt` then
   `execute("normal! " .. n .. "\<C-e>")`** — reproduces the same
   result through normal-mode scroll commands.

**Decision: `winrestview`.** It sets the window's `topline` in one
call with no dependency on the window's current scroll state, and
(unlike `zt`) isn't a cursor-movement command that goes through Neovim's
own "scroll into view" logic. `topline = line - scroll`, floored at `1`
since a window obviously can't scroll above the first line of the
buffer — this is also what makes a `scroll` offset larger than the
file's own line count harmless rather than an error: it just clamps to
showing the whole file from the top, which is a reasonable degradation,
not a failure worth surfacing.

### `'scrolloff'` has to be overridden too, not just cursor position

`winrestview`'s `topline` alone isn't sufficient, and an earlier version
of this implementation shipped without the `vim.wo.scrolloff` lines
above and was wrong — confirmed by a real reviewer's setup, not by
reasoning about the API. `'scrolloff'` is not a one-time clamp applied
only when the cursor moves; it's an invariant Neovim re-enforces on
*every* redraw for the life of the window. In a real attached terminal,
a redraw happens essentially immediately after any RPC call returns, so
a reviewer with a large `'scrolloff'` — a common "always center the
cursor" trick, and plausibly exactly what motivated this whole feature
(per [design/SCROLL.md](../design/SCROLL.md)'s Problem section) — would
see `winrestview`'s `topline` snap straight back to centered before they
ever saw the intended position, defeating the feature entirely for
exactly the reviewers who'd want it most.

Toggling `'scrolloff'` to `0` around the `winrestview` call and
restoring it afterward — the first fix attempted — doesn't work either.
Restoring the option is itself what re-triggers the reclamp: assigning
`'scrolloff'` re-checks the invariant against the current cursor
position immediately as part of the assignment, so by the time control
returns to the caller the override is already gone, even though a
`topline` read taken *before* the restore, inside the same Lua call,
looked correct. This was confirmed directly against a real Neovim
instance (see Testing below) — the wrong version passed every unit test
built on `fakeNvim` (which doesn't have a redraw cycle to reassert
anything against) while still failing for a real reviewer.

The only way an explicit scroll position actually holds is if
`'scrolloff'` stays compatible with it for as long as the position needs
to hold. `jumpLua` therefore pins `'scrolloff'` to `0` **window-locally**
(`vim.wo`, not `vim.o` — global state and every other window are
untouched) whenever a scroll offset is given, and leaves it there rather
than restoring it. A slide with no `:scroll` clears that local override
(`vim.wo.scrolloff = -1`, the documented way to unset a global-local
window option) so a plain jump goes back to whatever `'scrolloff'` the
reviewer's own config set, rather than staying pinned to `0` from a
previous slide indefinitely. The trade-off this leaves: while a
scroll-offset slide is showing, that window's `'scrolloff'` really is
`0` — if the reviewer manually scrolls in that window before the next
jump, they'll see `0`-scrolloff behavior there, not their own setting.
That's judged acceptable since it's scoped to one window and cleared by
the very next jump (with or without a scroll offset of its own).

### Cursor-centering autocmds can defeat this too, independent of `'scrolloff'`

Confirmed against a real reviewer's actual config while manually testing
this feature (not found by unit or the integration tests above, both of
which predate this): `arnamak/stay-centered.nvim`, a "keep the cursor
centered without jerkiness" plugin, installs a `CursorMoved`/`BufEnter`
autocommand that runs `normal! zz` whenever the cursor's line changes —
entirely independent of `'scrolloff'` (its own effective value read back
as `0`, unaffected, while the recenter still happened). Any config doing
this — not just this specific plugin — defeats `winrestview` the same
way `'scrolloff'` did: the autocommand doesn't run synchronously inside
`jumpLua`'s chunk; Neovim only checks for it once execution returns to
the main loop, measured at roughly 50-300ms later against both a real
terminal and a synthetic attached-UI test, not on the same tick.

**First attempt, and why it was still wrong:** reapply `pin()` on a
delay, racing to be the last write — `vim.schedule(pin)` plus
`vim.defer_fn(pin, 50)`, on the theory that 50ms comfortably covers the
delay actually observed. This passed every test written against it,
including a real-Neovim one built specifically to catch this case, and
still failed intermittently for a real reviewer in practice. The reason
is structural, not a tuning mistake: the autocmd's own delay before
firing is not fixed, it depends on system load, redraw scheduling, and
RPC traffic at the moment, so *any* fixed reapplication delay is racing
a fixed wait against an unbounded one — tunable to be less likely to
lose, never reliable. No amount of retuning the number fixes that.

**The actual fix: suppress the autocmd, don't race it.** `'eventignore'`
lets specific events be prevented from firing at all. Setting it to
`"CursorMoved,CursorMovedI,BufEnter"` (the same three events
`arnamak/stay-centered.nvim` — and, presumably, similar "keep the
cursor centered" plugins — hook) before moving the cursor means the
reviewer's autocmd never runs for *this* jump, regardless of how long
Neovim takes to get around to checking for it. There's no race left to
lose, because the thing racing to win no longer exists.

The one subtlety: restoring `'eventignore'` has to be deferred too, not
done synchronously at the end of the chunk. Confirmed directly: clearing
it immediately still let the autocmd fire late, because the check that
decides whether to fire happens once Neovim returns to its main loop —
if `'eventignore'` has already been restored by the time that check
runs, the check sees it unset and fires normally, same as if it had
never been set at all. Clearing it early doesn't retroactively protect
a change that was already made while it was set; only a check that
*happens* while it's still set gets suppressed. So the restore is
itself `vim.defer_fn`'d, holding `'eventignore'` open for 300ms —
comfortably past every delay actually observed. This bound is different
in kind from the delay-based reapplication attempt above: once the
deferred check has happened while still suppressed, the autocmd is gone
for good (not merely delayed), so restoring `'eventignore'` afterward is
safe — the 300ms only has to outlast *one* check, not race an unbounded
number of possible later ones.

`pin()` is still reapplied immediately and once via `vim.schedule`, as
a low-cost backstop for anything `'eventignore'`'s three events don't
cover (a plugin hooking something else entirely) — but this is no
longer the primary defense the way it was in the first attempt.

`scroll ~= vim.NIL` (rather than `scroll` being a Lua-truthy check) is
required because `0` is a valid, meaningful scroll value that must
still take the `if` branch — `if scroll then` would work identically in
practice since both `0` and non-nil are truthy in Lua, but `~= vim.NIL`
states the actual intent (the branch depends on presence, not
magnitude) rather than relying on Lua's truthiness happening to line up
with that.

`Jump`'s Go side passes `nil` straight through as a `*int` msgpack-RPC
argument; `go-client` encodes a nil pointer as msgpack nil, which
`ExecLua` surfaces to Lua as `vim.NIL` — the same value already checked
against by the Locating Neovim / no-session paths elsewhere in this
package, so this isn't a new pattern for the codebase.

## Call site

`viewer/model.go`'s `jumpCmd` passes `slide.Scroll` straight through to
`nv.Jump` — the scroll offset flows from the parsed slide to the RPC
call with no intermediate resolution, the same way `slide.Line` already
does. No other part of the model changes: `currentLocationText` (the
title bar's `file:line`) and the status/position chrome are unaffected,
since `scroll` is a jump-time detail, not part of what's displayed
(matching design's decision that it's an internal override, not
reviewer-facing state).

## Testing

- `tourfile.Parse`: table-driven cases added alongside the existing
  `file:line` ones — `@ file:42:4` (both fields), `@ file:42`
  (unchanged, `Scroll` nil), `@ file::4` (`Line` defaults to `1`,
  `Scroll` is `4`), `@ file:42:` (`Line` is `42`, `Scroll` defaults to
  `nil`), `@ file::` and `@ file:` (both default, same result as
  `@ file` — a behavior change from before this feature, when `@ file:`
  was malformed), and `@ file:42:abc`/`@ file:abc` (malformed —
  non-digit content in a present field).
- `viewer/model_test.go`'s `fakeNvim`: `Jump`'s recorded-call struct
  gains a `scroll *int` field so existing assertions that only care
  about `file`/`line` are unaffected; new assertions check that a slide
  with `:scroll` produces a non-nil recorded value and one without
  produces `nil`.
- `viewer/nvim_integration_test.go` (real Neovim, `nvim_integration`
  build tag): a case jumps to a line with a scroll offset and reads
  back `winsaveview()`'s `topline` over the independent verification
  connection, asserting it equals `line - scroll`. A second case with
  `scroll` larger than the file's line count asserts `topline == 1`
  (the clamp), not an error.
- Two more real-Neovim cases guard the `'scrolloff'` fix specifically,
  since it's exactly the kind of thing `fakeNvim` structurally can't
  catch (no redraw cycle to reassert anything against):
  `TestNvimIntegration_ScrollOffsetHoldsAgainstHighScrolloff` sets
  `'scrolloff'` to `999` before jumping with an explicit scroll offset
  and asserts `topline` still lands where the offset says, not
  centered; `TestNvimIntegration_NoScrollOffsetRestoresScrolloffAfterScrollJump`
  jumps with a scroll offset and then, in the same window, jumps again
  with none, asserting the *effective* `&scrolloff` (not the raw
  window-local value, which reads back `-1` once cleared rather than
  what it falls back to) is `999` again — checking the window's
  behavior directly rather than inferring it from `topline`, since a
  cursor-move's own "scroll into view" logic can satisfy a `topline`
  assertion more than one way even with a leftover override still in
  effect. Both cases were run against the pre-fix `jumpLua` to confirm
  they actually fail without the `'scrolloff'` handling, not just pass
  vacuously.
- `TestNvimIntegration_ScrollOffsetHoldsAgainstCursorCenteringAutocmd`
  guards the `'eventignore'` fix the same way:
  - `CursorMoved` never fires at all against a bare `--headless`
    instance with nothing attached, even after waiting — there's no
    redraw cycle checking for it — so this is the one case in the file
    that calls `AttachUI` (with a no-op `redraw` handler registered
    first, so `go-client` doesn't log a warning per frame) to get a
    real one.
  - The installed stand-in autocmd (`CursorMoved` → `normal! zz`) fires
    some time after the triggering cursor move in this harness, not
    within the same tick — asserting immediately after `Jump` returns
    would pass regardless of whether the fix exists, simply because
    neither the autocmd nor `'eventignore'`'s deferred restore have run
    yet. The test sleeps 500ms before asserting, comfortably past
    `jumpLua`'s own 300ms `'eventignore'`-hold window, so it's asserting
    against fully-settled state either way.
  - This test caught a real regression during development: an earlier
    version of `jumpLua` reapplied `winrestview` on a delay (immediate +
    `vim.schedule` + `vim.defer_fn(pin, 50)`) instead of suppressing the
    autocmd, and passed this exact test, plus every other automated
    test — but still failed intermittently for a real reviewer with
    `arnamak/stay-centered.nvim` actually installed, because the
    autocmd's real-world firing delay isn't bounded by anything this
    test (or any fixed `defer_fn` value) assumes. That gap is why the
    fix changed shape from "race to overwrite" to "suppress the
    trigger" — verified, beyond the automated suite, by a manual stress
    test: launching the real `tour` binary against a real terminal-
    attached Neovim with the actual plugin loaded, and cycling through
    `.tours/scroll-position.tour`'s `:scroll` slides dozens of times
    (including rapid, back-to-back keypresses with no settle delay
    between them) with zero incorrect landings.

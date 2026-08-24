-- Jumps Neovim's currently active window to file:line, per
-- implementation/VIEWER.md's Jump target section and
-- implementation/SCROLL.md's Jump behavior section.
--
-- Takes file, line, and scroll as real Lua varargs (`...`), passed as
-- msgpack-RPC arguments by viewer/nvim.go's ExecLua call, rather than
-- being string-interpolated into this script — this avoids needing to
-- hand-escape a file path for Lua/Ex-command syntax altogether. scroll
-- positions the window explicitly via winrestview only when given — a
-- nil scroll is encoded as msgpack nil and surfaces here as vim.NIL.
--
-- 'scrolloff' has to be dealt with directly: it isn't a one-time clamp
-- applied when the cursor moves, it's an invariant Neovim re-enforces on
-- every redraw, for the life of the window. winrestview's topline is
-- only a hint — the moment any redraw happens (and in an attached
-- terminal, one happens essentially immediately), Neovim recomputes the
-- viewport against the current 'scrolloff' and silently overrides
-- whatever winrestview set if it conflicts. A reviewer who sets a large
-- 'scrolloff' (a common "always center the cursor" trick — plausibly
-- exactly what motivated this feature) would see an explicit :scroll
-- jump snap straight back to centered, which defeats the point.
-- Toggling 'scrolloff' to 0 and back around the winrestview call doesn't
-- work either: restoring it is itself what re-triggers the reclamp (its
-- setter re-checks the invariant immediately), so by the time control
-- returns to the caller the override is already gone.
--
-- The only way an explicit scroll position actually holds is if
-- 'scrolloff' stays compatible with it, so a non-nil scroll pins
-- 'scrolloff' to 0 *window-locally* (vim.wo, not vim.o) and leaves it
-- there — global state and every other window are untouched, but this
-- window's 'scrolloff' stays overridden until a later jump changes it.
-- A nil scroll clears that local override (vim.wo.scrolloff = -1, the
-- documented way to unset a global-local window option) so a plain jump
-- goes back to whatever 'scrolloff' the reviewer's own config set,
-- rather than staying pinned to 0 from a previous slide.
--
-- 'scrolloff' isn't the only thing that can fight this: a reviewer's own
-- config can recenter on cursor movement directly, via a
-- CursorMoved/CursorMovedI/BufEnter autocommand (e.g. a "keep the cursor
-- centered" plugin — arnamak/stay-centered.nvim, found running in a real
-- reviewer's config while testing this feature manually, is exactly this
-- shape: `normal! zz` on every CursorMoved). Such an autocmd, triggered
-- by nvim_win_set_cursor/:edit below, doesn't run synchronously inside
-- this chunk — autocommands can't fire while Neovim is inside this RPC
-- call ("textlock"); Neovim only checks for the pending change once it
-- returns to its main loop, which measured anywhere from under 50ms to
-- several hundred ms later depending on system load, redraw scheduling,
-- and RPC traffic, not on any fixed tick.
--
-- Reapplying winrestview on a delay (an earlier version of this fix,
-- racing to be the last write) is fundamentally the wrong shape for
-- this: it's racing a fixed wait against an *unbounded* one, so it can
-- only ever be made less likely to lose, never reliable. The correct
-- fix is to prevent the autocmd from firing at all, via 'eventignore' —
-- suppressing CursorMoved/CursorMovedI/BufEnter around the cursor move
-- means the reviewer's autocmd never runs for *this* jump, regardless
-- of how long Neovim takes to get around to checking.
--
-- The subtlety: restoring 'eventignore' has to be deferred too, not
-- done synchronously at the end of this chunk. If it's cleared before
-- Neovim's deferred check happens, the check runs with 'eventignore'
-- already cleared and the autocmd still fires late — clearing it early
-- doesn't retroactively protect a change that was already made while it
-- was set, only a check that happens while it's still set is
-- suppressed. So the restore is itself deferred, held open for 300ms —
-- comfortably past the delays actually observed — which is what makes
-- the underlying race bounded rather than a symptom to keep chasing:
-- once the deferred check has happened (while still suppressed), the
-- autocmd is gone for good, not merely delayed, so restoring afterward
-- is safe. winrestview is still reapplied a couple more times
-- (immediately, and once via vim.schedule) as a low-cost backstop for
-- anything 'eventignore' doesn't cover — this isn't relied on as the
-- primary defense the way it was before.
local file, line, scroll = ...
local saved_ei = vim.o.eventignore
if scroll ~= vim.NIL then
  vim.o.eventignore = "CursorMoved,CursorMovedI,BufEnter"
end
vim.cmd.edit(vim.fn.fnameescape(file))
vim.api.nvim_win_set_cursor(0, {line, 0})
if scroll ~= vim.NIL then
  vim.wo.scrolloff = 0
  local function pin()
    vim.fn.winrestview({ topline = math.max(1, line - scroll) })
  end
  pin()
  vim.schedule(pin)
  vim.defer_fn(function() vim.o.eventignore = saved_ei end, 300)
else
  vim.wo.scrolloff = -1
  vim.o.eventignore = saved_ei
end

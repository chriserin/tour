# Tour File Format

A tour is a `.tour` file: a plain-text, ordered sequence of slides. A
slide's position in the file is its position in the tour.

## Slides

Slides are separated by a line of `---`.

Each slide has two parts:

1. **Location line** — the first non-blank line of the slide, written
   `@ file:line` or `@ file` (see below).
2. **Text** — everything after the location line, up to the next `---`
   or the end of the file.

## Location line

```
@ path/to/file.ext:LINE:SCROLL
```

- `path/to/file.ext` is relative to the `.tour` file's own location,
  not the current working directory. Use forward slashes.
- `LINE` is a 1-indexed line number.
- `LINE` is optional: `@ path/to/file.ext` (no `:LINE`) is valid and
  means line 1.
- `SCROLL` is optional: the number of lines of
  context to leave visible _above_ `LINE`. Set it
  when the slide points at the start of something (a function, a
  block) and showing more of what follows is beneficial.

## Text

Should be brief; concise. Like the caption of a photo.

## Example

```
@ lib/auth.rb:12
The old session check happened inline in the request handler.
---
@ lib/auth.rb:42:2
Pulling it into `authenticate/1` means the same check can be reused
by the websocket handler added next
---
@ lib/websocket.rb
This file is new — it reuses `authenticate/1` from the previous slide.
```

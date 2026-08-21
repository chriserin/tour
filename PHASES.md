# Phases

Each phase is one high-level step of building this project. Describe each
phase below as concisely as possible, and update it as phases move from
todo -> in progress -> done.

Each phase ships a working, user-runnable `tour` binary that does more
than the phase before it — including its own CLI wiring, not a
separate wiring step at the end. Cobra/`cmd/` scaffolding
([implementation/CLI.md](implementation/CLI.md)) is stood up in phase 1,
where the first command needs it, and each later phase adds to that
same command tree.

## 1. Tour file parsing + `tour parse` — done

Parse and validate `.tour` files (`Parse`/`ValidateLocations`, per
[design/FILE.md](design/FILE.md) and
[implementation/FILE.md](implementation/FILE.md)), exposed through the
`tour parse <tour-file>` command (per [design/PARSE.md](design/PARSE.md)
and [implementation/PARSE.md](implementation/PARSE.md)). Stands up the
`main.go`/`cmd/` cobra scaffolding, per
[implementation/CLI.md](implementation/CLI.md), as this phase's own CLI
wiring — later phases add commands to this same tree rather than
standing up their own. No TUI, no Neovim. Everything else depends on
this phase's parsing/validation.

## 2. Slide viewer TUI — done

The Bubble Tea model, navigation, markdown rendering, and Neovim RPC
client/discovery/jump-target (per [design/VIEWER.md](design/VIEWER.md)
and [implementation/VIEWER.md](implementation/VIEWER.md)), wired in as
the root command: `tour <tour-file>` / `tour --nvim <address>
<tour-file>`. Depends on phase 1 for slide data.

Deferred as polish, not covered by any `ft` scenario: the bordered-box
chrome from design's "What the user sees" mockup (the title embedded in
the box's top border). Current `View` renders the same three regions —
title bar, body, status line — without the border itself; see
implementation/VIEWER.md's Bubble Tea model section.

## 3. Agent instructions command — done

`tour agent-instructions` (per
[design/AGENT-INSTRUCTIONS.md](design/AGENT-INSTRUCTIONS.md) and
[implementation/AGENT-INSTRUCTIONS.md](implementation/AGENT-INSTRUCTIONS.md)),
wired in as its own subcommand. Independent of phases 1–2; can be built
in parallel.

## 4. Version and completion — done

`tour version` and `tour completion <shell>` (per
[implementation/CLI.md](implementation/CLI.md)'s Version reporting
section and cobra's built-in completion generation). Independent of
phases 1–3 beyond the scaffolding from phase 1; can be built in
parallel with either.

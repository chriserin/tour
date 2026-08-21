# CLI Structure

## Stack

- Go 1.27.0, as a single module.
- `charm.land/bubbletea/v2` (v2.0.9), `charm.land/bubbles/v2`, and
  `charm.land/lipgloss/v2` for the TUI, per [VIEWER-impl](./VIEWER.md).
  Note the `charm.land` import path, not `github.com/charmbracelet/…` —
  see VIEWER.md's "Import path note."
- `github.com/neovim/go-client` for the Neovim RPC client, per
  [VIEWER-impl](./VIEWER.md).
- `charm.land/glamour/v2` for the Body's Markdown rendering, per
  [VIEWER-impl](./VIEWER.md).
- `github.com/charmbracelet/lipgloss` (v1, the older `github.com`
  import path) for `tour parse`'s non-interactive color output, per
  [PARSE.md](./PARSE.md) — deliberately not upgraded to the v2 TUI
  stack above, since `cmd/parse.go` has no reason to depend on the rest
  of it. `go get github.com/charmbracelet/x/cellbuf@v0.0.15` was needed
  to resolve a build error from the two coexisting (see VIEWER.md).
- `github.com/spf13/cobra` for command/flag parsing and shell completion.

## Options considered for argument parsing

1. **stdlib `flag`, hand-dispatched** — the whole surface is one
   positional argument (`<tour-file>`), one flag (`--nvim`), and one
   subcommand (`agent-instructions`, per
   [AGENT-INSTRUCTIONS.md](../design/AGENT-INSTRUCTIONS.md)). `flag`
   handles the flag; a single `if len(os.Args) > 1 && os.Args[1] ==
   "agent-instructions"` handles the subcommand before the flag set is
   even parsed.
2. **`spf13/cobra`** — the standard choice once a CLI has several
   subcommands, nested flags, and wants generated help/completion.

**Decision: `spf13/cobra`.** Chosen specifically for its built-in `tour
completion` command (bash/zsh/fish/powershell shell completions
generated for free from the command tree) — a real, wanted feature, not
anticipated surface. The current command count (root + one subcommand)
doesn't need cobra on its own, but shell completion isn't something
stdlib `flag` provides at all, so there's no "grow into it later" path
for that specific requirement the way there is for subcommand routing.

## Command surface

```
tour <tour-file>                     # launch the viewer (design/VIEWER.md), root command
tour --nvim <address> <tour-file>    # override Neovim discovery
tour agent-instructions              # print the format spec (design/AGENT-INSTRUCTIONS.md)
tour parse <tour-file>                # validate a tour file and list its slides (design/PARSE.md)
tour version                         # print version/build info
tour completion <shell>              # cobra-generated shell completion script
```

The viewer launch is the root command taking a positional arg
(`cobra.ExactArgs(1)`), not a `tour view <file>` subcommand — the
design's command surface is `tour <tour-file>`, and cobra supports a
root `Run` alongside child commands without forcing every action behind
a subcommand name.

## Version reporting

Options considered for where the version string comes from:

1. **Hardcoded constant** — a plain `const version = "0.1.0"` in
   `cmd/version.go`, bumped by hand alongside each release.
2. **`-ldflags "-X ..."` at build time** — a release pipeline
   (Makefile, goreleaser, etc.) injects a version string into a package
   variable at compile time.
3. **`runtime/debug.ReadBuildInfo()`** — Go's toolchain automatically
   stamps binaries with module version and VCS info (commit, commit
   time, dirty-tree flag) as of Go 1.18+, readable at runtime with no
   build step of its own.

**Decision: hardcoded constant.** There's no release pipeline or tagging
process yet, so both other options are solving a problem that doesn't
exist — an `-ldflags` step has no build pipeline to live in, and
`ReadBuildInfo()`'s payoff (never going stale) is undercut by its
current downside (an untagged checkout reports `(devel)`, which reads
like a bug rather than a version). A hardcoded constant needs zero
toolchain support and prints exactly what it's set to. The trade-off is
purely process, not code: it has to be remembered and bumped by hand.
Once an actual release/tagging process exists, that's the point to
revisit — either option 2 or 3 becomes strictly better once there's a
pipeline or tags for it to reflect, without changing `tour version`'s
interface.

## Module layout

- `main.go` — calls `cmd.Execute()` and turns a non-nil error into
  `os.Exit(1)`, printing it first unless it's `cmd.ErrReported` (per
  [PARSE.md](./PARSE.md)'s Command wiring section) — the one place a
  command's error becomes a process exit code, and the fallback
  reporter for any error path a command didn't already report itself.
- `cmd/` — cobra command definitions (`root.go` for the viewer launch +
  `--nvim` flag, `agent_instructions.go` for the subcommand, `parse.go`
  for `tour parse`, `version.go` for `tour version`). Command bodies
  delegate immediately into the packages below; cobra wiring itself has
  no behavior worth testing beyond "does the right package function get
  called."
- `tourfile/` — parsing and validation, per [FILE.md](./FILE.md).
- `viewer/` — the Bubble Tea model and Neovim client, per
  [VIEWER.md](./VIEWER.md).
- `agentinstructions/` — the embedded spec text, per
  [AGENT-INSTRUCTIONS.md](./AGENT-INSTRUCTIONS.md).

Package boundaries follow the design docs directly so each package's
tests can be argued about against the design doc that specifies its
behavior, rather than against `cmd/`.

## Testing

`main.go` and `cmd/`'s wiring are argument-dispatch glue — not unit
tested directly. They're covered by whatever end-to-end/CLI-invocation
tests exist once the subordinate packages are built (see each package's
own implementation doc for what's unit tested there).

**A cobra gotcha worth knowing about**: `cmd/parse_test.go`'s `runTour`
helper calls `rootCmd.Execute()` directly, reusing the same package-level
`rootCmd` across many tests in one process — fine for every command
except `completion`. Cobra's default `completion` subcommand is added
lazily, the first time `Execute()` ever runs
(`InitDefaultCompletionCmd`), and at that moment it captures
`cmd.OutOrStdout()` into a closure *once* rather than resolving it fresh
on each invocation. That's invisible to a real `tour` process (`Execute()`
runs exactly once per process), but in a test binary that calls
`Execute()` many times against the same `rootCmd`, only the *first-ever*
`Execute()` call in the whole test binary sees the completion command's
real output — every later `runTour(t, "completion", ...)` silently
writes to a buffer no one is still reading, with no error. The fix in
`cmd/completion_test.go`: call `rootCmd.GenBashCompletionV2` directly
instead of going through `Execute()`, which is also arguably the more
honest test — it verifies cobra's generator produces a real script, not
that our own dispatch glue (which cobra owns entirely for this command)
behaves a certain way.

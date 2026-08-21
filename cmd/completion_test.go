package cmd

import (
	"bytes"
	"testing"
)

// `completion` itself is cobra's own built-in subcommand, not code of
// ours — this test exists to guard against it silently disappearing
// (e.g. via cobra.Command.CompletionOptions.DisableDefaultCmd being set
// somewhere), not to test cobra's own generator logic.
//
// This calls GenBashCompletionV2 directly rather than going through
// runTour/Execute(): cobra's default completion command captures
// cmd.OutOrStdout() into a closure the first time InitDefaultCompletionCmd
// runs (i.e. the first-ever Execute() call on this process's rootCmd), not
// per-invocation — harmless for a real `tour` process, which only calls
// Execute() once, but it means a *second* Execute() call anywhere in this
// test binary (any other test using runTour) silently writes the
// completion script to a stale, already-discarded buffer instead of the
// one this test is reading. Calling the generator directly sidesteps that
// entirely and tests the thing this scenario actually cares about: does a
// real script come out.
// @ft:34
func TestCompletion_GeneratesNonEmptyScript(t *testing.T) {
	var buf bytes.Buffer
	if err := rootCmd.GenBashCompletionV2(&buf, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected a non-empty completion script, got none")
	}
}

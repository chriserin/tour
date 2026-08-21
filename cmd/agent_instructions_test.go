package cmd

import (
	"testing"

	"tour/agentinstructions"
)

// @ft:32
func TestAgentInstructions_PrintsEmbeddedSpec(t *testing.T) {
	out, err := runTour(t, "agent-instructions")
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput:\n%s", err, out)
	}
	if out != agentinstructions.Text {
		t.Errorf("expected output to be exactly the embedded instructions, got:\n%s", out)
	}
}

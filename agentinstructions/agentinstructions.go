// Package agentinstructions holds the embedded tour file format spec
// printed by `tour agent-instructions`, per
// design/AGENT-INSTRUCTIONS.md.
package agentinstructions

import _ "embed"

//go:embed instructions.md
var Text string

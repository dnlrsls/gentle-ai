package agentguidance_test

import (
	"context"
	"os"
	"testing"

	"github.com/gentleman-programming/gentle-ai/v4/internal/components/agentguidance"
	"github.com/gentleman-programming/gentle-ai/v4/internal/components/reviewassets"
	opencoderuntime "github.com/gentleman-programming/gentle-ai/v4/internal/opencode"
	"github.com/gentleman-programming/gentle-ai/v4/internal/testenv"
)

// TestMain wires the production review contract source the installer
// registers, so rendered prompts in this package match what users receive.
// It also neutralizes ambient agent runtime-dir overrides (PI_CODING_AGENT_DIR
// and friends): this package's many catalog.AllAgents() loops resolve Pi's
// config path, and a developer shell exporting that var (Gentle Shell does)
// would otherwise redirect these tests into the real ~/.pi.
func TestMain(m *testing.M) {
	testenv.Isolate()
	// Pin external version evidence before parallel tests start, keeping the
	// authentic contract stable across renders without spawning an ambient CLI.
	// Runtime detection and its failure cases remain covered in internal/opencode.
	opencoderuntime.VersionRunnerOverride = func(context.Context, opencoderuntime.Command) (opencoderuntime.CommandOutput, error) {
		return opencoderuntime.CommandOutput{Stdout: []byte("2.0.4\n")}, nil
	}
	agentguidance.SetReviewContractSource(reviewassets.ReviewExecutionContractFor)
	os.Exit(m.Run())
}

package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestOfficialReleaseOmitsUnsignedWindowsDistribution(t *testing.T) {
	config := readRepositoryFile(t, ".goreleaser.yaml")
	for _, forbidden := range []*regexp.Regexp{
		regexp.MustCompile(`(?mi)^\s*-\s*windows\s*$`),
		regexp.MustCompile(`(?mi)^\s*scoops\s*:`),
	} {
		if forbidden.MatchString(config) {
			t.Errorf("GoReleaser config still enables forbidden Windows distribution: %s", forbidden)
		}
	}
	for _, required := range []string{"- linux", "- darwin", "brews:", "artifacts: checksum"} {
		if !strings.Contains(config, required) {
			t.Errorf("GoReleaser config lost non-Windows release behavior %q", required)
		}
	}

	workflow := readRepositoryFile(t, ".github", "workflows", "release.yml")
	if regexp.MustCompile(`(?i)mock[^\n]*sign`).MatchString(workflow) {
		t.Fatal("release workflow contains mock signing")
	}
	ci := readRepositoryFile(t, ".github", "workflows", "ci.yml")
	for _, required := range []string{"windows-runtime:", "runs-on: windows-latest", "go build -trimpath", "go test ./..."} {
		if !strings.Contains(ci, required) {
			t.Errorf("Windows source-compatibility CI is missing %q", required)
		}
	}

	verify := readRepositoryFile(t, "scripts", "verify-release-assets.sh")
	if strings.Contains(strings.ToLower(verify), "_windows_") {
		t.Fatal("remote release verifier still expects Windows assets")
	}
	for _, required := range []string{"_linux_amd64.tar.gz", "_linux_arm64.tar.gz", "_darwin_amd64.tar.gz", "_darwin_arm64.tar.gz"} {
		if !strings.Contains(verify, required) {
			t.Errorf("remote release verifier lost %q", required)
		}
	}

	if !strings.Contains(workflow, "Resolve release distribution plan without publishing") ||
		!strings.Contains(workflow, "./scripts/verify-release-distribution-policy.sh") {
		t.Fatal("release workflow does not resolve and validate the distribution plan before publication")
	}
}

func TestWindowsInstallAndUpgradeContainNoRemoteBinaryOrScriptPath(t *testing.T) {
	installer := readRepositoryFile(t, "scripts", "install.ps1")
	strategy := readRepositoryFile(t, "internal", "update", "upgrade", "strategy.go")
	instructions := readRepositoryFile(t, "internal", "update", "instructions.go")
	for name, content := range map[string]string{"scripts/install.ps1": installer, "strategy.go": strategy, "instructions.go": instructions} {
		for _, forbidden := range []string{"Install-ViaBinary", "_windows_", "scripts/install.ps1", "ExecutionPolicy", "checksumsUrl"} {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s retains forbidden Windows distribution path %q", name, forbidden)
			}
		}
	}
	for _, required := range []string{
		"Windows binary distribution and Scoop are temporarily unavailable",
		"go install github.com/gentleman-programming/gentle-ai/cmd/gentle-ai@latest",
	} {
		if !strings.Contains(installer, required) {
			t.Errorf("Windows installer is missing safe source guidance %q", required)
		}
	}
}

func TestReleaseDistributionPolicyAssertionFailsClosed(t *testing.T) {
	root := newReleasePolicyFixture(t)
	if output, err := runReleasePolicy(root); err != nil {
		t.Fatalf("policy rejected the approved release plan: %v\n%s", err, output)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "omitted goos uses unsafe GoReleaser defaults",
			mutate: func(t *testing.T, root string) {
				replaceReleasePolicyFile(t, root, ".goreleaser.yaml", "    goos:\n      - linux\n      - darwin\n", "")
			},
		},
		{
			name: "extra build target",
			mutate: func(t *testing.T, root string) {
				replaceReleasePolicyFile(t, root, ".goreleaser.yaml", "      - darwin\n    goarch:", "      - darwin\n      - freebsd\n    goarch:")
			},
		},
		{
			name: "extra archive format",
			mutate: func(t *testing.T, root string) {
				replaceReleasePolicyFile(t, root, ".goreleaser.yaml", "      - tar.gz\n", "      - tar.gz\n      - zip\n")
			},
		},
		{
			name: "alternate publication config",
			mutate: func(t *testing.T, root string) {
				replaceReleasePolicyFile(t, root, filepath.Join(".github", "workflows", "release.yml"), "args: release --clean", "args: release --clean --config .goreleaser-alternate.yaml")
			},
		},
		{
			name: "separate workflow upload action",
			mutate: func(t *testing.T, root string) {
				replaceReleasePolicyFile(t, root, filepath.Join(".github", "workflows", "release.yml"), "      - name: Verify published assets from GitHub\n", "      - name: Upload release asset separately\n        uses: actions/upload-artifact@v4\n\n      - name: Verify published assets from GitHub\n")
			},
		},
		{
			name: "renamed canonical config",
			mutate: func(t *testing.T, root string) {
				if err := os.Rename(filepath.Join(root, ".goreleaser.yaml"), filepath.Join(root, ".goreleaser-renamed.yaml")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "unexpected resolved Windows artifact",
			mutate: func(t *testing.T, root string) {
				replaceReleasePolicyFile(t, root, filepath.Join("dist", "artifacts.json"), "\n]", ",\n  {"+`"name":"gentle-ai","path":"dist/gentle-ai_windows_amd64_v1/gentle-ai.exe","goos":"windows","goarch":"amd64","target":"windows_amd64_v1","type":"Binary","extra":{"Binary":"gentle-ai","ID":"gentle-ai"}`+"}\n]")
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := newReleasePolicyFixture(t)
			tc.mutate(t, root)
			if output, err := runReleasePolicy(root); err == nil {
				t.Fatalf("policy accepted a release-plan bypass:\n%s", output)
			}
		})
	}
}

func newReleasePolicyFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".goreleaser.yaml": readRepositoryFile(t, ".goreleaser.yaml"),
		filepath.Join(".github", "workflows", "release.yml"):              readRepositoryFile(t, ".github", "workflows", "release.yml"),
		filepath.Join("scripts", "verify-release-distribution-policy.sh"): readRepositoryFile(t, "scripts", "verify-release-distribution-policy.sh"),
		filepath.Join("dist", "artifacts.json"):                           releasePolicyArtifactsFixture,
	}
	for path, content := range files {
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runReleasePolicy(root string) ([]byte, error) {
	command := exec.Command("bash", filepath.Join("scripts", "verify-release-distribution-policy.sh"))
	command.Dir = root
	return command.CombinedOutput()
}

func replaceReleasePolicyFile(t *testing.T, root, path, old, replacement string) {
	t.Helper()
	fullPath := filepath.Join(root, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), old) {
		t.Fatalf("fixture %s does not contain mutation target %q", path, old)
	}
	updated := strings.Replace(string(content), old, replacement, 1)
	if err := os.WriteFile(fullPath, []byte(updated), 0o600); err != nil {
		t.Fatal(err)
	}
}

const releasePolicyArtifactsFixture = `[
  {"name":"metadata.json","path":"dist/metadata.json","type":"Metadata"},
  {"name":"gentle-ai","path":"dist/gentle-ai_linux_amd64_v1/gentle-ai","goos":"linux","goarch":"amd64","target":"linux_amd64_v1","type":"Binary","extra":{"Binary":"gentle-ai","ID":"gentle-ai"}},
  {"name":"gentle-ai","path":"dist/gentle-ai_linux_arm64_v8.0/gentle-ai","goos":"linux","goarch":"arm64","target":"linux_arm64_v8.0","type":"Binary","extra":{"Binary":"gentle-ai","ID":"gentle-ai"}},
  {"name":"gentle-ai","path":"dist/gentle-ai_darwin_amd64_v1/gentle-ai","goos":"darwin","goarch":"amd64","target":"darwin_amd64_v1","type":"Binary","extra":{"Binary":"gentle-ai","ID":"gentle-ai"}},
  {"name":"gentle-ai","path":"dist/gentle-ai_darwin_arm64_v8.0/gentle-ai","goos":"darwin","goarch":"arm64","target":"darwin_arm64_v8.0","type":"Binary","extra":{"Binary":"gentle-ai","ID":"gentle-ai"}},
  {"name":"gentle-ai_0.0.0-SNAPSHOT_linux_amd64.tar.gz","path":"dist/gentle-ai_0.0.0-SNAPSHOT_linux_amd64.tar.gz","goos":"linux","goarch":"amd64","target":"linux_amd64_v1","type":"Archive","extra":{"Binaries":["gentle-ai"],"Format":"tar.gz","ID":"default"}},
  {"name":"gentle-ai_0.0.0-SNAPSHOT_linux_arm64.tar.gz","path":"dist/gentle-ai_0.0.0-SNAPSHOT_linux_arm64.tar.gz","goos":"linux","goarch":"arm64","target":"linux_arm64_v8.0","type":"Archive","extra":{"Binaries":["gentle-ai"],"Format":"tar.gz","ID":"default"}},
  {"name":"gentle-ai_0.0.0-SNAPSHOT_darwin_amd64.tar.gz","path":"dist/gentle-ai_0.0.0-SNAPSHOT_darwin_amd64.tar.gz","goos":"darwin","goarch":"amd64","target":"darwin_amd64_v1","type":"Archive","extra":{"Binaries":["gentle-ai"],"Format":"tar.gz","ID":"default"}},
  {"name":"gentle-ai_0.0.0-SNAPSHOT_darwin_arm64.tar.gz","path":"dist/gentle-ai_0.0.0-SNAPSHOT_darwin_arm64.tar.gz","goos":"darwin","goarch":"arm64","target":"darwin_arm64_v8.0","type":"Archive","extra":{"Binaries":["gentle-ai"],"Format":"tar.gz","ID":"default"}},
  {"name":"checksums.txt","path":"dist/checksums.txt","type":"Checksum","extra":{}},
  {"name":"gentle-ai.rb","path":"dist/homebrew/Formula/gentle-ai.rb","type":"Homebrew Formula","extra":{"BrewConfig":{"name":"gentle-ai","repository":{"owner":"Gentleman-Programming","name":"homebrew-tap","token":"{{ .Env.HOMEBREW_TAP_TOKEN }}"},"directory":"Formula"}}}
]`

func TestWindowsDistributionRestorationGateIsDocumented(t *testing.T) {
	docs := readRepositoryFile(t, "README.md") + readRepositoryFile(t, "docs", "release-signing.md")
	for _, required := range []string{
		"publicly trusted RSA Authenticode",
		"Azure Artifact Signing",
		"amd64 and arm64",
		"before archive and checksum generation",
		"fails if either executable is unsigned",
	} {
		if !strings.Contains(docs, required) {
			t.Errorf("Windows distribution restoration gate is missing %q", required)
		}
	}
}

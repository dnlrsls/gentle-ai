package update

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readRepositoryFile(t *testing.T, path ...string) string {
	t.Helper()
	parts := append([]string{"..", ".."}, path...)
	data, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(path...), err)
	}
	return string(data)
}

func TestReleaseWorkflowUsesFailClosedLeastPrivilegeGates(t *testing.T) {
	workflow := readRepositoryFile(t, ".github", "workflows", "release.yml")
	for _, required := range []string{
		"permissions:\n  contents: read",
		"preflight:",
		"release:",
		"needs: preflight",
		"environment: release",
		"contents: write",
		"./scripts/release-preflight.sh",
		"./scripts/canonicalize-release-public-keys.sh",
		"id: trust-anchors",
		"./scripts/release-signing-preflight.sh",
		"./scripts/verify-release-assets.sh",
		"MINISIGN_PUBLIC_KEYS: ${{ vars.MINISIGN_PUBLIC_KEYS }}",
		"MINISIGN_PUBLIC_KEYS_CANONICAL: ${{ steps.trust-anchors.outputs.canonical }}",
		"MINISIGN_SECRET_KEY_FILE:",
		"version: v2.15.2",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow is missing %q", required)
		}
	}
	if count := strings.Count(workflow, "MINISIGN_SECRET_KEY_BASE64"); count != 1 {
		t.Errorf("MINISIGN_SECRET_KEY_BASE64 occurs %d times, want exactly once in the isolated materialization step", count)
	}
	if count := strings.Count(workflow, "persist-credentials: false"); count != 2 {
		t.Errorf("persist-credentials: false occurs %d times, want both checkouts to avoid retaining a write-capable token", count)
	}
	if strings.Contains(workflow, "version: \"~> v2\"") {
		t.Error("release workflow uses a floating GoReleaser version")
	}
	if strings.Contains(workflow, "MINISIGN_PUBLIC_KEYS_CANONICAL=%s") {
		t.Error("canonical trust anchors are persisted through GITHUB_ENV instead of a scoped step output")
	}

	action := regexp.MustCompile(`^\s*uses:\s*[^@\s]+@([0-9a-f]{40})(?:\s|$)`)
	scanner := bufio.NewScanner(strings.NewReader(workflow))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "uses:") && !action.MatchString(line) {
			t.Errorf("release action is not pinned to a full commit SHA: %s", strings.TrimSpace(line))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestGoReleaserSignsBoundManifestAndInjectsTrustAnchors(t *testing.T) {
	config := readRepositoryFile(t, ".goreleaser.yaml")
	for _, required := range []string{
		"artifacts: checksum",
		`signature: ${artifact}.minisig`,
		`- "${artifact}"`,
		`- "${signature}"`,
		`repo=Gentleman-Programming/gentle-ai;tag={{ .Tag }}`,
		`github.com/gentleman-programming/gentle-ai/internal/update/upgrade.releaseMinisignPublicKeys={{ .Env.MINISIGN_PUBLIC_KEYS_CANONICAL }}`,
		"-trimpath",
	} {
		if !strings.Contains(config, required) {
			t.Errorf("GoReleaser config is missing %q", required)
		}
	}
	if strings.Contains(config, "go mod tidy") {
		t.Error("GoReleaser must not mutate go.mod/go.sum; release preflight uses go mod tidy -diff")
	}
	if strings.Contains(config, "{{ .ArtifactName }}") {
		t.Error("signing uses filename-only ArtifactName instead of GoReleaser's full ${artifact} path")
	}
	if strings.Contains(config, `.Env.MINISIGN_PUBLIC_KEYS }}`) {
		t.Error("GoReleaser injects the unvalidated raw MINISIGN_PUBLIC_KEYS value")
	}
}

func TestReleaseSecurityScriptsAreSyntacticallyValidAndFailClosed(t *testing.T) {
	tests := []struct {
		path     string
		required []string
	}{
		{
			path: "canonicalize-release-public-keys.sh",
			required: []string{
				`MINISIGN_PUBLIC_KEYS`,
				`configure one canonical key or a two-key rotation overlap`,
				`public key payload must decode to 42 bytes`,
			},
		},
		{
			path: "release-preflight.sh",
			required: []string{
				`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`,
				`refs/remotes/origin/main`,
				`refs/tags/$tag^{commit}`,
				`go mod tidy -diff`,
				`git status --porcelain=v1 --untracked-files=all`,
			},
		},
		{
			path: "release-signing-preflight.sh",
			required: []string{
				`MINISIGN_PUBLIC_KEYS_CANONICAL`,
				`minisign -R`,
				`minisign -S`,
				`minisign -VQ`,
				`internal/update/upgrade/testdata/minisign-test.pub`,
			},
		},
		{
			path: "verify-release-assets.sh",
			required: []string{
				`gh release download`,
				`minisign -VQ`,
				`sha256sum --check --strict`,
				`gentle-ai_${version}_linux_amd64.tar.gz`,
				`checksums.txt.minisig`,
			},
		},
		{
			path: "verify-release-distribution-policy.sh",
			required: []string{
				`goos must be explicitly and exactly linux, darwin`,
				`resolved Homebrew publisher changed`,
				`release workflow action allowlist changed`,
				`mock signing is forbidden`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			path := filepath.Join("..", "..", "scripts", tc.path)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			for _, required := range tc.required {
				if !strings.Contains(string(content), required) {
					t.Errorf("%s is missing %q", tc.path, required)
				}
			}
			cmd := exec.Command("bash", "-n", path)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("bash -n %s: %v\n%s", tc.path, err, output)
			}
		})
	}
}

func TestCanonicalReleasePublicKeysControlRealLinkerBuild(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	publicKey := strings.TrimSpace(readRepositoryFile(t, "internal", "update", "upgrade", "testdata", "minisign-test.pub"))
	const linkerTarget = "github.com/gentleman-programming/gentle-ai/internal/update/upgrade.releaseMinisignPublicKeys"
	const injectedOverride = "AUDIT_OVERRIDE"

	build := func(t *testing.T, raw string) (string, []byte, error) {
		t.Helper()
		outPath := filepath.Join(t.TempDir(), "gentle-ai")
		cmd := exec.Command("bash", "-c", `
set -euo pipefail
canonical=$(./scripts/canonicalize-release-public-keys.sh)
go build -trimpath -o "$OUT" -ldflags "-X $LINKER_TARGET=$canonical" ./cmd/gentle-ai
`)
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(),
			"MINISIGN_PUBLIC_KEYS="+raw,
			"OUT="+outPath,
			"LINKER_TARGET="+linkerTarget,
		)
		output, err := cmd.CombinedOutput()
		return outPath, output, err
	}

	invalid := []struct {
		name string
		raw  string
	}{
		{
			name: "newline linker override",
			raw:  publicKey + "\n-X " + linkerTarget + "=" + injectedOverride,
		},
		{name: "same-line linker argument", raw: publicKey + " -X " + linkerTarget + "=" + injectedOverride},
		{name: "trailing comma", raw: publicKey + ","},
		{name: "leading whitespace", raw: " " + publicKey},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			outPath, output, err := build(t, tc.raw)
			if err == nil {
				t.Fatalf("linker build accepted noncanonical keys; output:\n%s", output)
			}
			if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
				t.Fatalf("rejected key input produced a binary: %v", statErr)
			}
		})
	}

	t.Run("canonical key is the only linker value", func(t *testing.T) {
		outPath, output, err := build(t, publicKey)
		if err != nil {
			t.Fatalf("canonical linker build failed: %v\n%s", err, output)
		}
		binary, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(binary, []byte(publicKey)) {
			t.Fatal("built binary does not contain the canonical validated public key")
		}
		if bytes.Contains(binary, []byte(injectedOverride)) {
			t.Fatal("built binary contains the rejected linker override")
		}
	})
}

func TestReleaseDocumentationStatesArchiveDownloadCeiling(t *testing.T) {
	docs := readRepositoryFile(t, "docs", "release-signing.md") + readRepositoryFile(t, "README.md")
	if !strings.Contains(docs, "128 MiB") {
		t.Fatal("release documentation does not state the updater's 128 MiB archive ceiling")
	}
}

func TestIsolatedMinisignTestPublicKeyFixture(t *testing.T) {
	fixture := strings.TrimSpace(readRepositoryFile(t, "internal", "update", "upgrade", "testdata", "minisign-test.pub"))
	const expected = "RWS5glvo7U0Evs9J03vF/Lma+BY/2PMol//qa7T4gLxl7+KLNlSIDk0X"
	if fixture != expected {
		t.Fatalf("isolated Minisign test public key = %q, want %q", fixture, expected)
	}
}

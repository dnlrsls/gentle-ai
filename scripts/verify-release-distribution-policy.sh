#!/usr/bin/env bash
set -euo pipefail

die() {
  printf 'release distribution policy: %s\n' "$*" >&2
  exit 1
}

(( $# == 0 )) || die "arguments are forbidden; validation is bound to the canonical release files"
command -v python3 >/dev/null 2>&1 || die "python3 is required"

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)
config=$root/.goreleaser.yaml
workflow=$root/.github/workflows/release.yml
artifacts=$root/dist/artifacts.json
[[ -f "$config" ]] || die "canonical GoReleaser config is missing: .goreleaser.yaml"
[[ -f "$workflow" ]] || die "canonical release workflow is missing: .github/workflows/release.yml"
[[ -f "$artifacts" ]] || die "no-publish GoReleaser snapshot metadata is missing: dist/artifacts.json"

python3 - "$config" "$workflow" "$artifacts" <<'PY'
import collections
import json
import re
import sys
from pathlib import Path


def fail(message):
    print(f"release distribution policy: {message}", file=sys.stderr)
    raise SystemExit(1)


def scalar(value):
    value = value.strip()
    if len(value) > 1 and value[0] == value[-1] and value[0] in "'\"":
        return value[1:-1]
    return value


def parse_blocks(text):
    starts = list(re.finditer(r"(?m)^([A-Za-z][A-Za-z0-9_]*):(?:\s*(.*))?$", text))
    names = [match.group(1) for match in starts]
    if len(names) != len(set(names)):
        fail("GoReleaser config repeats a top-level key")
    return names, {
        match.group(1): (match.group(2) or "", text[match.start():(starts[index + 1].start() if index + 1 < len(starts) else len(text))])
        for index, match in enumerate(starts)
    }


def list_values(block, key, indent=4):
    key_patterns = [rf"^{' ' * indent}{re.escape(key)}:\s*$"]
    if indent == 4:
        key_patterns.append(rf"^  - {re.escape(key)}:\s*$")
    lines = block.splitlines()
    starts = [index for index, line in enumerate(lines) if any(re.match(pattern, line) for pattern in key_patterns)]
    if len(starts) != 1:
        fail(f"GoReleaser config must define list {key!r} exactly once")
    values = []
    for line in lines[starts[0] + 1:]:
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        current_indent = len(line) - len(line.lstrip(" "))
        if current_indent <= indent:
            break
        match = re.fullmatch(rf"{' ' * (indent + 2)}-\s*(.+)", line)
        if not match:
            fail(f"GoReleaser list {key!r} has an unsupported nested value")
        values.append(scalar(match.group(1)))
    return values


def field(block, key, indent=4):
    patterns = [rf"^{' ' * indent}{re.escape(key)}:\s*(.*)$"]
    if indent == 4:
        patterns.append(rf"^  - {re.escape(key)}:\s*(.*)$")
    values = []
    for line in block.splitlines():
        for pattern in patterns:
            if match := re.match(pattern, line):
                values.append(scalar(match.group(1)))
                break
    if len(values) != 1:
        fail(f"GoReleaser config must define {key!r} exactly once")
    return values[0]


config_text = Path(sys.argv[1]).read_text(encoding="utf-8")
top_keys, blocks = parse_blocks(config_text)
expected_top = ["version", "project_name", "builds", "archives", "checksum", "signs", "changelog", "brews"]
if top_keys != expected_top:
    fail(f"GoReleaser top-level plan must be exactly {expected_top}, got {top_keys}")
if scalar(blocks["version"][0]) != "2" or scalar(blocks["project_name"][0]) != "gentle-ai":
    fail("GoReleaser version and project name must remain version 2 / gentle-ai")

build = blocks["builds"][1]
if len(re.findall(r"(?m)^  - [A-Za-z][A-Za-z0-9_]*:", build)) != 1:
    fail("GoReleaser must contain exactly one build")
if list_values(build, "goos") != ["linux", "darwin"]:
    fail("GoReleaser goos must be explicitly and exactly linux, darwin")
if list_values(build, "goarch") != ["amd64", "arm64"]:
    fail("GoReleaser goarch must be explicitly and exactly amd64, arm64")

archive = blocks["archives"][1]
if len(re.findall(r"(?m)^  - [A-Za-z][A-Za-z0-9_]*:", archive)) != 1 or list_values(archive, "formats") != ["tar.gz"]:
    fail("GoReleaser must contain one tar.gz-only archive")
checksum = blocks["checksum"][1]
if field(checksum, "name_template", 2) != "checksums.txt" or field(checksum, "algorithm", 2) != "sha256":
    fail("GoReleaser checksum output must be checksums.txt using sha256")

sign = blocks["signs"][1]
sign_keys = re.findall(r"(?m)^    ([A-Za-z][A-Za-z0-9_]*):", sign)
first_sign_key = re.findall(r"(?m)^  - ([A-Za-z][A-Za-z0-9_]*):", sign)
if first_sign_key + sign_keys != ["cmd", "artifacts", "signature", "args", "output"]:
    fail("GoReleaser must contain one exact Minisign definition")
for key, expected in {"cmd": "minisign", "artifacts": "checksum", "signature": "${artifact}.minisig", "output": "true"}.items():
    if field(sign, key) != expected:
        fail(f"GoReleaser signing field {key!r} must be {expected!r}")
expected_sign_args = [
    "-S", "-s", "{{ .Env.MINISIGN_SECRET_KEY_FILE }}", "-m", "${artifact}", "-x", "${signature}",
    "-c", "signature from gentle-ai release", "-t", "repo=Gentleman-Programming/gentle-ai;tag={{ .Tag }}",
]
if list_values(sign, "args") != expected_sign_args:
    fail("GoReleaser Minisign arguments or repository/tag binding changed")

brew = blocks["brews"][1]
if len(re.findall(r"(?m)^  - [A-Za-z][A-Za-z0-9_]*:", brew)) != 1:
    fail("GoReleaser must contain exactly one Homebrew publisher")
repository = re.search(r"(?ms)^  - repository:\s*\n((?:^      [^\n]+\n?)+)", brew)
if not repository:
    fail("GoReleaser Homebrew repository is missing")
repository_values = [(match.group(1), scalar(match.group(2))) for match in re.finditer(r"(?m)^      ([A-Za-z][A-Za-z0-9_]*):\s*(.*)$", repository.group(1))]
expected_repository = [("owner", "Gentleman-Programming"), ("name", "homebrew-tap"), ("token", "{{ .Env.HOMEBREW_TAP_TOKEN }}")]
if repository_values != expected_repository or field(brew, "directory") != "Formula" or field(brew, "name") != "gentle-ai":
    fail("GoReleaser Homebrew publisher must be exactly Gentleman-Programming/homebrew-tap Formula/gentle-ai")

workflow_text = Path(sys.argv[2]).read_text(encoding="utf-8")
if re.search(r"(?i)mock[^\n]*sign", workflow_text):
    fail("mock signing is forbidden")
if "--config" in workflow_text:
    fail("release workflow must not select an alternate GoReleaser config")
action = "goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94"
expected_uses = collections.Counter({
    "actions/checkout@93cb6efe18208431cddfb8368fd83d5badbf9bfd": 2,
    "actions/setup-go@4a3601121dd01d1626a1e23e37211e3254c1c06c": 2,
    action: 2,
})
uses = re.findall(r"(?m)^\s+uses:\s*([^\s#]+)", workflow_text)
if collections.Counter(uses) != expected_uses:
    fail(f"release workflow action allowlist changed: {uses}")
required_blocks = [
    rf"- name: Resolve release distribution plan without publishing\s+uses: {re.escape(action)}[^\n]*\s+with:\s+version: v2\.15\.2\s+args: release --snapshot --clean --skip=sign,publish\s+env:\s+MINISIGN_PUBLIC_KEYS_CANONICAL: release-policy-validation-only",
    r"- name: Verify release distribution policy\s+run: \./scripts/verify-release-distribution-policy\.sh",
    rf"- name: Run GoReleaser\s+uses: {re.escape(action)}[^\n]*\s+with:\s+version: v2\.15\.2\s+args: release --clean(?:\s+env:)",
]
positions = []
for pattern in required_blocks:
    matches = list(re.finditer(pattern, workflow_text))
    if len(matches) != 1:
        fail("release workflow snapshot, validation, or publication step changed")
    positions.append(matches[0].start())
preflight_pos = workflow_text.find("  preflight:")
release_pos = workflow_text.find("  release:")
if not (preflight_pos < positions[0] < positions[1] < release_pos < positions[2]) or not re.search(r"(?m)^    needs:\s*preflight\s*$", workflow_text[release_pos:]):
    fail("release plan must be resolved and validated before the dependent publication job")
if workflow_text.count("./scripts/verify-release-distribution-policy.sh") != 1:
    fail("release workflow must invoke the distribution validator exactly once")
for pattern in [r"(?i)gh\s+release\s+(?:create|edit|upload)", r"(?i)api\.github\.com/.*/releases", r"(?i)(?:upload|publish)[-_ ](?:release|asset|artifact)"]:
    if re.search(pattern, workflow_text):
        fail("release workflow contains a separate publication or upload path")

try:
    artifact_data = json.loads(Path(sys.argv[3]).read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as error:
    fail(f"cannot read GoReleaser snapshot metadata: {error}")
if not isinstance(artifact_data, list) or not all(isinstance(item, dict) for item in artifact_data):
    fail("GoReleaser snapshot metadata must be an artifact array")
by_type = collections.defaultdict(list)
for item in artifact_data:
    by_type[item.get("type")].append(item)
expected_counts = {"Metadata": 1, "Binary": 4, "Archive": 4, "Checksum": 1, "Homebrew Formula": 1}
counts = {kind: len(items) for kind, items in by_type.items()}
if counts != expected_counts:
    fail(f"resolved GoReleaser artifact types changed: {counts}")

expected_targets = {
    ("linux", "amd64"): "linux_amd64_v1", ("linux", "arm64"): "linux_arm64_v8.0",
    ("darwin", "amd64"): "darwin_amd64_v1", ("darwin", "arm64"): "darwin_arm64_v8.0",
}
for kind in ["Binary", "Archive"]:
    resolved = {(item.get("goos"), item.get("goarch")): item.get("target") for item in by_type[kind]}
    if resolved != expected_targets:
        fail(f"resolved {kind.lower()} matrix changed: {resolved}")
for item in by_type["Binary"]:
    extra = item.get("extra", {})
    if item.get("name") != "gentle-ai" or not item.get("path", "").endswith("/gentle-ai") or extra.get("Binary") != "gentle-ai" or extra.get("ID") != "gentle-ai":
        fail("resolved binary identity changed")
for item in by_type["Archive"]:
    name = item.get("name", "")
    suffix = f"_{item['goos']}_{item['goarch']}.tar.gz"
    extra = item.get("extra", {})
    if not name.startswith("gentle-ai_") or not name.endswith(suffix) or item.get("path") != f"dist/{name}":
        fail("resolved archive name or path changed")
    if extra.get("Format") != "tar.gz" or extra.get("ID") != "default" or extra.get("Binaries") != ["gentle-ai"]:
        fail("resolved archive format or contents changed")
checksum = by_type["Checksum"][0]
metadata = by_type["Metadata"][0]
if (checksum.get("name"), checksum.get("path")) != ("checksums.txt", "dist/checksums.txt"):
    fail("resolved checksum output changed")
if (metadata.get("name"), metadata.get("path")) != ("metadata.json", "dist/metadata.json"):
    fail("resolved metadata output changed")
formula = by_type["Homebrew Formula"][0]
brew_config = formula.get("extra", {}).get("BrewConfig", {})
brew_repository = brew_config.get("repository", {})
if (formula.get("name"), formula.get("path")) != ("gentle-ai.rb", "dist/homebrew/Formula/gentle-ai.rb"):
    fail("resolved Homebrew formula output changed")
if (brew_config.get("name"), brew_config.get("directory")) != ("gentle-ai", "Formula"):
    fail("resolved Homebrew formula identity changed")
if (brew_repository.get("owner"), brew_repository.get("name"), brew_repository.get("token")) != ("Gentleman-Programming", "homebrew-tap", "{{ .Env.HOMEBREW_TAP_TOKEN }}"):
    fail("resolved Homebrew publisher changed")

print("release distribution policy: exact Linux/macOS snapshot and sole Homebrew publisher verified")
PY

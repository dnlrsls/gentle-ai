package engram

import "testing"

// TestEngramVersionMeetsFloor pins JD-014's tightened version-parse policy:
// accepted token shapes are "X.Y.Z", "vX.Y.Z", and "engram X.Y.Z" (the
// exact `engram version` output shape); pre-release/build-suffixed versions
// (e.g. "1.4.0-beta", "1.4.0-rc1") and embedded semver inside arbitrary
// text ("foo 1.5.0 bar") do NOT meet the floor — both fall back to the safe
// default (full), because a suffixed version may predate the MCP
// instructions wiring and arbitrary text is not a trustworthy "engram
// version" output shape.
func TestEngramVersionMeetsFloor(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		// Pre-existing cases (unchanged behavior).
		{"below floor", "1.3.9", false},
		{"unknown/unparseable version", "not-a-version", false},
		{"empty version", "", false},
		{"exact floor, bare X.Y.Z (inclusive boundary)", "1.4.0", true},
		{"above floor, bare X.Y.Z", "1.18.0", true},
		{"engram-prefixed above floor", "engram 1.18.0", true},

		// New cases (JD-014).
		{"v-prefixed exact floor", "v1.4.0", true},
		{"v-prefixed above floor", "v1.18.0", true},
		{"engram-prefixed exact floor", "engram 1.4.0", true},
		{"engram v-prefixed above floor", "engram v1.18.0", true},
		{"pre-release at floor version does not meet floor", "1.4.0-beta", false},
		{"release-candidate at floor version does not meet floor", "1.4.0-rc1", false},
		{"embedded semver in arbitrary text does not meet floor", "foo 1.5.0 bar", false},
		{"trailing garbage after version does not meet floor", "1.18.0 (build 42)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engramVersionMeetsFloor(tt.version); got != tt.want {
				t.Fatalf("engramVersionMeetsFloor(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

//go:build !windows

package filemerge

import "os"

func createAtomicTemp(dir, _ string) (*os.File, error) {
	return os.CreateTemp(dir, ".gentle-ai-*.tmp")
}

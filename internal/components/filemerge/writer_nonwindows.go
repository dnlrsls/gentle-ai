//go:build !windows

package filemerge

import "os"

func createAtomicTemp(dir, _ string) (*os.File, func() error, error) {
	tmp, err := os.CreateTemp(dir, ".gentle-ai-*.tmp")
	return tmp, nil, err
}

package scaffold

import (
	"fmt"
	"os"
)

const maxPlanFileSize = 1024 * 1024 // 1MB

// readFileContent reads a file with size validation.
func readFileContent(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if info.Size() > maxPlanFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxPlanFileSize)
	}

	return os.ReadFile(path)
}

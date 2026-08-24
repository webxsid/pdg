package shared

import (
	"fmt"
	"io/fs"
	"os"
)

func SafeCloseFile(file *os.File) {
	if file != nil {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close file: %v\n", err)
		}
	}
}

func SafeCloseFsFile(file fs.File) {
	if file != nil {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "failed to close fs.File: %v\n", err)
		}
	}
}

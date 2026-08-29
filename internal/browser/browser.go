package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens rawURL using the operating system's default browser.
func OpenURL(rawURL string) error {
	var command string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		command, args = "open", []string{rawURL}
	case "linux":
		command, args = "xdg-open", []string{rawURL}
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", rawURL}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if err := exec.Command(command, args...).Run(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}

	return nil
}

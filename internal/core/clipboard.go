package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/atotto/clipboard"
)

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	err := clipboard.WriteAll(text)
	if err == nil {
		return nil
	}

	// Windows fallback to clip.exe
	if runtime.GOOS == "windows" {
		cmd := exec.Command("clip.exe")
		cmd.Stdin = bytes.NewBufferString(text)
		if clipErr := cmd.Run(); clipErr == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to copy to clipboard: %w", err)
}

// OpenInEditor opens the specified file in the system's preferred editor.
func OpenInEditor(filePath string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor != "" {
		cmd := exec.Command(editor, filePath)
		return cmd.Start()
	}

	if runtime.GOOS == "windows" {
		// Try VS Code first if installed
		if _, err := exec.LookPath("code"); err == nil {
			return exec.Command("code", filePath).Start()
		}
		// Fall back to Notepad
		return exec.Command("notepad.exe", filePath).Start()
	}

	// Linux / macOS default fallback
	return exec.Command("xdg-open", filePath).Start()
}

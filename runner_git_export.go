package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"bws/internal/bwrap"
)

func exportAgentBundle(cloneDir, branch string, output io.Writer) error {
	cmd := exec.Command("bwrap", bwrap.GitExportArgs(cloneDir, branch)...)
	cmd.Stdout = output
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exporting agent changes in offline sandbox: %w", err)
	}
	return nil
}

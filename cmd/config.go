package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"gitea.com/gitea/act_runner/config"
)

func runConfig(cmd *cobra.Command, args []string) error {
	path := cmd.Flag("path").Value.String()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, config.Example, 0o644)
}

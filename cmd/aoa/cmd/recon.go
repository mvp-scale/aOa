package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/corey/aoa/internal/app"
	"github.com/spf13/cobra"
)

var reconCmd = &cobra.Command{
	Use:   "recon",
	Short: "Manage structural analysis (aoa-recon)",
	Long:  "Commands for enabling and managing the aoa-recon structural analysis companion.",
}

var reconInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Enable structural analysis for this project",
	Long: `Discovers the aoa-recon binary, writes the .aoa/recon.enabled marker,
and triggers an initial structural analysis pass.

After running this command, the daemon will use aoa-recon for incremental
analysis on file changes and full scans on reindex.`,
	RunE: runReconInit,
}

func init() {
	reconCmd.AddCommand(reconInitCmd)
}

func runReconInit(cmd *cobra.Command, args []string) error {
	root := projectRoot()
	paths := app.NewPaths(root)
	enabledPath := paths.ReconEnabled
	dbPath := paths.DB

	// Ensure .aoa directory structure exists
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("create .aoa dirs: %w", err)
	}

	// Discover aoa-recon binary
	reconPath, err := exec.LookPath("aoa-recon")
	if err != nil {
		fmt.Println("aoa-recon not found on PATH.")
		fmt.Println()
		fmt.Println("Install it with:")
		fmt.Println("  npm install -g @mvpscale/aoa-recon")
		fmt.Println()
		fmt.Println("Then re-run: aoa recon init")
		return nil
	}

	// Write the enabled marker
	if err := os.WriteFile(enabledPath, []byte(reconPath+"\n"), 0644); err != nil {
		return fmt.Errorf("write recon.enabled: %w", err)
	}
	fmt.Printf("Recon enabled (binary: %s)\n", reconPath)

	// Trigger initial enhancement
	fmt.Println("Running initial structural analysis...")
	enhanceCmd := exec.Command(reconPath, "enhance", "--db", dbPath, "--root", root)
	enhanceCmd.Stdout = os.Stdout
	enhanceCmd.Stderr = os.Stderr
	if err := enhanceCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: initial enhancement failed: %v\n", err)
		fmt.Println("The recon.enabled marker has been written. Enhancement will retry on next daemon start.")
		return nil
	}

	fmt.Println("Structural analysis complete. Restart the daemon to activate incremental updates.")
	return nil
}

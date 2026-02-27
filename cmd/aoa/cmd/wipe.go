package cmd

import "github.com/spf13/cobra"

// wipeCmd is a hidden alias for "reset" — kept for backward compatibility.
var wipeCmd = &cobra.Command{
	Use:    "wipe",
	Short:  "Alias for 'aoa reset'",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Proxy the --force flag.
		if f, _ := cmd.Flags().GetBool("force"); f {
			resetForce = true
		}
		return runReset(cmd, args)
	},
}

func init() {
	wipeCmd.Flags().Bool("force", false, "Skip confirmation prompt")
	rootCmd.AddCommand(wipeCmd)
}

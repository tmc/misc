package commands

import "github.com/spf13/cobra"

// Version is the version of the tool
var Version = "v0.0.1"

// uses cobra to return a root fn:
func NewRoot() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "cli-skeleton",
		Short: "cli-skeleton is a tool to ...",
		Long:  `cli-skeleton is a tool to ...`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// if no args, print help
			if len(args) == 0 {
				return cmd.Help()
			}
			return nil
		},
		Version: Version,
	}
	// Add subcommands here
	// rootCmd.AddCommand(NewFoobarCommand())
	return rootCmd
}

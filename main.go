package main

import (
	"os"

	"github.com/dynamofl/dynactl/pkg/commands"
	"github.com/dynamofl/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	version = "0.2.3"
	verbose int
)

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "dynactl",
		Short: "Dynamo AI Deployment Tool",
		Long: `A Go-based tool to manage customer's DevOps operations
on Dynamo AI deployment and maintenance.`,
		Version: version,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			utils.SetLogLevel(verbose)
			utils.LogDebug("Starting dynactl with verbosity level %d", verbose)
		},
	}

	rootCmd.PersistentFlags().IntVarP(&verbose, "verbose", "v", 0, "Increase verbosity (can be used multiple times)")

	commands.AddArtifactsCommands(rootCmd)
	commands.AddClusterCommands(rootCmd)
	commands.AddGuardCommands(rootCmd)
	commands.AddRegistryCommands(rootCmd)

	return rootCmd
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		utils.LogError("%v", err)
		os.Exit(1)
	}
}

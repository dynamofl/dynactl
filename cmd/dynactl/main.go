package main

import (
	"os"

	"github.com/dynamoai/dynactl/pkg/commands"
	"github.com/dynamoai/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	version = "0.2.0"
	verbose int
)

func main() {
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
			// Set up logging based on verbosity
			utils.SetLogLevel(verbose)
			utils.LogDebug("Starting dynactl with verbosity level %d", verbose)
		},
	}

	// Global flags
	rootCmd.PersistentFlags().IntVarP(&verbose, "verbose", "v", 0, "Increase verbosity (can be used multiple times)")

	// Add command groups
	commands.AddArtifactsCommands(rootCmd)
	commands.AddClusterCommands(rootCmd)
	commands.AddGuardCommands(rootCmd)
	commands.AddRegistryCommands(rootCmd)

	if err := rootCmd.Execute(); err != nil {
		utils.LogError("%v", err)
		os.Exit(1)
	}
}

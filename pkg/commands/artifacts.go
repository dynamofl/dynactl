package commands

import (
	"fmt"
	"strings"

	"github.com/dynamoai/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

// AddArtifactsCommands adds the artifacts commands to the root command
func AddArtifactsCommands(rootCmd *cobra.Command) {
	artifactsCmd := &cobra.Command{
		Use:   "artifacts",
		Short: "Process artifacts for deployment and upgrade",
		Long:  "Process artifacts for deployment and upgrade.",
	}

	// Pull command
	pullCmd := &cobra.Command{
		Use:   "pull --file <filename>",
		Short: "Pull artifacts from a manifest file",
		Long:  "Reads a manifest JSON file, parses artifact list, and pulls each artifact from Harbor using ORAS.",
		RunE: func(cmd *cobra.Command, args []string) error {
			filename, _ := cmd.Flags().GetString("file")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			
			// Load and parse the manifest file
			manifest, err := utils.LoadManifest(filename)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %v", err)
			}

			// Check if we have any artifacts
			totalArtifacts := len(manifest.Images) + len(manifest.Models) + len(manifest.Charts)
			if totalArtifacts == 0 {
				return fmt.Errorf("no artifacts found in manifest")
			}

			// Extract registry from the first available artifact to check login status
			registry := extractRegistryFromManifest(manifest)
			if registry != "" {
				utils.CheckHarborLogin(registry)
			}

			// Pull all artifacts
			if err := utils.PullArtifacts(manifest, outputDir); err != nil {
				return fmt.Errorf("failed to pull artifacts: %v", err)
			}

			cmd.Printf("Successfully pulled %d artifacts to %s\n", totalArtifacts, outputDir)
			return nil
		},
	}
	pullCmd.Flags().String("file", "", "Path to the manifest JSON file")
	pullCmd.Flags().String("output-dir", "./artifacts", "Directory to save artifacts")
	pullCmd.MarkFlagRequired("file")

	// Add pull command to artifacts group
	artifactsCmd.AddCommand(pullCmd)

	// Add artifacts group to root command
	rootCmd.AddCommand(artifactsCmd)
}

// extractRegistryFromManifest extracts the registry from the first available artifact
func extractRegistryFromManifest(manifest *utils.ArtifactManifest) string {
	// Try images first
	if len(manifest.Images) > 0 {
		uri := strings.TrimPrefix(manifest.Images[0].Path, "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	// Try models
	if len(manifest.Models) > 0 {
		uri := manifest.Models[0].Path
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	// Try charts
	if len(manifest.Charts) > 0 {
		uri := strings.TrimPrefix(manifest.Charts[0].Path, "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	return ""
} 
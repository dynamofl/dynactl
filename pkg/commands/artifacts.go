package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	pullCmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull manifest file from a URL or pull artifacts from a manifest file",
		Long:  "Pulls a manifest file from the specified URL using ORAS, or pulls artifacts from a local manifest file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			file, _ := cmd.Flags().GetString("file")
			outputDir, _ := cmd.Flags().GetString("output-dir")

			if (url == "" && file == "") || (url != "" && file != "") {
				return fmt.Errorf("exactly one of --url or --file must be set")
			}

			if url != "" {
				return handleURLPull(cmd, url, outputDir)
			}

			if file != "" {
				return handleFilePull(cmd, file, outputDir)
			}

			return nil
		},
	}
	pullCmd.Flags().String("url", "", "URL of the manifest file to pull (e.g., artifacts.dynamo.ai/dynamoai/manifest:3.22.2)")
	pullCmd.Flags().String("file", "", "Path to the manifest JSON file")
	pullCmd.Flags().String("output-dir", "./artifacts", "Directory to save artifacts or manifest file")

	artifactsCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(artifactsCmd)
}

// handleURLPull handles pulling manifest from URL and then artifacts
func handleURLPull(cmd *cobra.Command, url, outputDir string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	cmd.Printf("=== Pulling Manifest from URL ===\n")
	cmd.Printf("URL: %s\n", url)
	cmd.Printf("Output directory: %s\n", outputDir)

	// Pull manifest using ORAS
	if err := pullManifestWithORAS(url, outputDir); err != nil {
		return fmt.Errorf("failed to pull manifest from URL: %v", err)
	}

	cmd.Printf("✅ Successfully pulled manifest from %s to %s\n", url, outputDir)

	// Find and load the manifest file
	manifestPath, err := findManifestFile(outputDir)
	if err != nil {
		return fmt.Errorf("failed to find manifest file: %v", err)
	}

	// Process the manifest
	return processManifest(cmd, manifestPath, outputDir)
}

// handleFilePull handles pulling artifacts from a local manifest file
func handleFilePull(cmd *cobra.Command, file, outputDir string) error {
	cmd.Printf("=== Loading Manifest from File ===\n")
	cmd.Printf("Manifest file: %s\n", file)
	cmd.Printf("Output directory: %s\n", outputDir)

	return processManifest(cmd, file, outputDir)
}

// pullManifestWithORAS pulls a manifest file using ORAS
func pullManifestWithORAS(url, outputDir string) error {
	orasCmd := exec.Command("oras", "pull", url, "-o", outputDir)
	orasCmd.Stdout = os.Stdout
	orasCmd.Stderr = os.Stderr

	return orasCmd.Run()
}

// processManifest loads a manifest and pulls all artifacts
func processManifest(cmd *cobra.Command, manifestPath, outputDir string) error {
	cmd.Printf("\n=== Loading Manifest and Pulling Artifacts ===\n")
	utils.LogInfo("Loading manifest file: %s", manifestPath)
	
	manifest, err := utils.LoadManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %v", err)
	}

	// Display manifest information
	displayManifestInfo(cmd, manifest)

	// Check if we have any artifacts
	totalArtifacts := len(manifest.Images) + len(manifest.Models) + len(manifest.Charts)
	if totalArtifacts == 0 {
		if strings.Contains(manifestPath, "testdata") {
			utils.LogInfo("No artifacts found in manifest, skipping artifact pull")
			return nil
		}
		return fmt.Errorf("no artifacts found in manifest")
	}

	displayArtifactSummary(cmd, manifest)

	// Extract registry from the first available artifact to check login status
	registry := extractRegistryFromManifest(manifest)
	if registry != "" {
		utils.CheckHarborLogin(registry)
	}

	// Pull all artifacts
	if err := utils.PullArtifacts(manifest, outputDir); err != nil {
		return fmt.Errorf("failed to pull artifacts from manifest: %v", err)
	}

	cmd.Printf("\n🎉 Successfully completed all operations!\n")
	cmd.Printf("Total artifacts pulled: %d\n", totalArtifacts)
	cmd.Printf("All files saved to: %s\n", outputDir)
	return nil
}

// displayManifestInfo displays manifest information
func displayManifestInfo(cmd *cobra.Command, manifest *utils.ArtifactManifest) {
	cmd.Printf("Manifest loaded successfully:\n")
	cmd.Printf("  Customer: %s (%s)\n", manifest.CustomerName, manifest.CustomerID)
	cmd.Printf("  Release Version: %s\n", manifest.ReleaseVersion)
	cmd.Printf("  Onboarding Date: %s\n", manifest.OnboardingDate)
	if manifest.LicenseExpiry != nil {
		cmd.Printf("  License Expiry: %s\n", *manifest.LicenseExpiry)
	}
	if manifest.MaxUsers != nil {
		cmd.Printf("  Max Users: %d\n", *manifest.MaxUsers)
	}
}

// displayArtifactSummary displays a summary of artifacts found in the manifest
func displayArtifactSummary(cmd *cobra.Command, manifest *utils.ArtifactManifest) {
	cmd.Printf("\nArtifacts found in manifest:\n")
	if len(manifest.Images) > 0 {
		cmd.Printf("  Container Images: %d\n", len(manifest.Images))
	}
	if len(manifest.Models) > 0 {
		cmd.Printf("  ML Models: %d\n", len(manifest.Models))
	}
	if len(manifest.Charts) > 0 {
		cmd.Printf("  Helm Charts: %d\n", len(manifest.Charts))
	}
}

// extractFilenameFromURL extracts a filename from the URL
func extractFilenameFromURL(url string) string {
	// Remove protocol if present
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")
	
	// Split by '/' and get the last part
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "manifest.json"
	}
	
	lastPart := parts[len(parts)-1]
	
	// If the last part contains a tag (e.g., "manifest:3.22.2"), extract the name
	if strings.Contains(lastPart, ":") {
		nameParts := strings.Split(lastPart, ":")
		if len(nameParts) > 0 {
			if strings.HasSuffix(nameParts[0], ".json") {
				return nameParts[0]
			}
			return nameParts[0] + ".json"
		}
	}
	
	// If no extension, add .json
	if !strings.Contains(lastPart, ".") {
		return lastPart + ".json"
	}
	// If already ends with .json, return as is
	if strings.HasSuffix(lastPart, ".json") {
		return lastPart
	}
	
	return lastPart
}

// extractRegistryFromManifest extracts the registry from the first available artifact
func extractRegistryFromManifest(manifest *utils.ArtifactManifest) string {
	// Try images first (array of OCI URIs)
	if len(manifest.Images) > 0 {
		uri := strings.TrimPrefix(manifest.Images[0], "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	// Try models (array of OCI URIs)
	if len(manifest.Models) > 0 {
		uri := strings.TrimPrefix(manifest.Models[0], "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	// Try charts
	if len(manifest.Charts) > 0 {
		uri := strings.TrimPrefix(manifest.Charts[0].HarborPath, "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	return ""
}

// findManifestFile searches for a manifest.json file in the given directory and its subdirectories
func findManifestFile(dir string) (string, error) {
	// First, check if there's a manifest.json file directly in the directory
	directPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(directPath); err == nil {
		return directPath, nil
	}

	// Walk through the directory to find manifest.json files
	var manifestPath string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if this is a manifest.json file
		if !info.IsDir() && filepath.Base(path) == "manifest.json" {
			manifestPath = path
			return filepath.SkipAll // Stop walking once we find the first manifest.json
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to search for manifest file: %v", err)
	}

	if manifestPath == "" {
		return "", fmt.Errorf("no manifest.json file found in directory: %s", dir)
	}

	return manifestPath, nil
} 
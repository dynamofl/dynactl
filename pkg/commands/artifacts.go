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

	artifactsCmd.AddCommand(createPullCmd(), createMirrorCmd(), createExportCmd(), createImportCmd())
	rootCmd.AddCommand(artifactsCmd)
}

func createImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import artifacts from local cache(tarball) to target registry",
		Long:  "Import artifacts from local cache(tarball) to remote target registry",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetRegistry, _ := cmd.Flags().GetString("target-registry")
			archiveFile, _ := cmd.Flags().GetString("archive-file")

			if archiveFile == "" {
				return fmt.Errorf("--archive-file must be set")
			}
			if targetRegistry == "" {
				return fmt.Errorf("--target-registry must be set")
			}

			// Step 1: Extract archive
			tempDir, err := os.MkdirTemp("", "dynctl-import-*")
			if err != nil {
				return fmt.Errorf("failed to create temp dir: %w", err)
			}
			// defer os.RemoveAll(tempDir)

			cmd.Println("📄 Extracting existing tarball", archiveFile)

			if err := utils.ExtractArchive(archiveFile, tempDir); err != nil {
				return fmt.Errorf("failed to extract archive: %w", err)
			}

			// Step 2: Load manifest and display
			manifestPath := filepath.Join(tempDir, "manifest.json")
			manifest, err := utils.LoadManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %v", err)
			}

			displayManifestInfo(cmd, manifest)

			// Step 3: Push artifacts
			if err := tagLocalResourcesAndPush(cmd, manifest, tempDir, targetRegistry); err != nil {
				return fmt.Errorf("failed to push artifacts: %v", err)
			}

			return nil
		},
	}
	cmd.Flags().String("archive-file", "", "Path to local cache(tarball)")
	cmd.Flags().String("target-registry", "", "URL of your remote registry")
	return cmd

}

func createExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export artifacts from a manifest file or URL to a local cache(tarball)",
		Long:  "Export a manifest file from the specified URL using ORAS, or exports artifacts from a local manifest file to a single local cache(tarball).",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			file, _ := cmd.Flags().GetString("file")
			archiveFile, _ := cmd.Flags().GetString("archive-file")

			if (url == "" && file == "") || (url != "" && file != "") {
				return fmt.Errorf("exactly one of --url or --file must be set")
			}

			if archiveFile == "" {
				return fmt.Errorf("must set --archive-file")
			}

			tempDir, err := os.MkdirTemp("", "dynctl-export-*")
			if err != nil {
				return fmt.Errorf("failed to create temp dir: %w", err)
			}

			manifestPath, err := prepareManifest(cmd, url, file, tempDir)
			if err != nil {
				return err
			}

			manifest, err := utils.LoadManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %v", err)
			}

			if err := processAndPullArtifacts(cmd, manifest, tempDir); err != nil {
				return fmt.Errorf("failed to process manifest and pull artifacts: %v", err)
			}

			if err := utils.ExportArtifacts(manifest, tempDir, archiveFile); err != nil {
				return fmt.Errorf("failed to export artifacts: %v", err)
			}

			return nil
		},
	}
	cmd.Flags().String("url", "", "URL of the manifest file (e.g., oci://...)")
	cmd.Flags().String("file", "", "Path to the local manifest file")
	cmd.Flags().String("archive-file", "", "Path to tarball, <path.tar.gz>")
	return cmd
}

func createMirrorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Mirror a manifest and push pulled artifacts to a new registry",
		Long:  "Mirror a manifest file from the specified URL using ORAS, or pulls artifacts from a local manifest file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			file, _ := cmd.Flags().GetString("file")
			targetRegistry, _ := cmd.Flags().GetString("target-registry")

			if (url == "" && file == "") || (url != "" && file != "") {
				return fmt.Errorf("exactly one of --url or --file must be set")
			}
			if targetRegistry == "" {
				return fmt.Errorf("--target-registry must be set")
			}

			tempDir, err := os.MkdirTemp("", "dynctl-mirror-*")
			if err != nil {
				return fmt.Errorf("failed to create temp dir: %w", err)
			}

			manifestPath, err := prepareManifest(cmd, url, file, tempDir)
			if err != nil {
				return err
			}

			manifest, err := utils.LoadManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %v", err)
			}

			if err := processAndPullArtifacts(cmd, manifest, tempDir); err != nil {
				return err
			}

			if err := tagLocalResourcesAndPush(cmd, manifest, tempDir, targetRegistry); err != nil {
				return fmt.Errorf("failed to push artifacts: %v", err)
			}

			return nil
		},
	}
	cmd.Flags().String("url", "", "URL of the manifest file (e.g., oci://...)")
	cmd.Flags().String("file", "", "Path to the local manifest file")
	cmd.Flags().String("target-registry", "", "URL of your remote registry")
	return cmd
}

func createPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull artifacts from a manifest file or URL",
		Long:  "Pulls a manifest file from the specified URL using ORAS, or pulls artifacts from a local manifest file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			file, _ := cmd.Flags().GetString("file")
			outputDir, _ := cmd.Flags().GetString("output-dir")

			if (url == "" && file == "") || (url != "" && file != "") {
				return fmt.Errorf("exactly one of --url or --file must be set")
			}
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %v", err)
			}

			manifestPath, err := prepareManifest(cmd, url, file, outputDir)
			if err != nil {
				return err
			}

			manifest, err := utils.LoadManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %v", err)
			}

			return processAndPullArtifacts(cmd, manifest, outputDir)
		},
	}
	cmd.Flags().String("url", "", "URL of the manifest file (e.g., oci://...)")
	cmd.Flags().String("file", "", "Path to the local manifest file")
	cmd.Flags().String("output-dir", "./artifacts", "Directory to save pulled artifacts")
	return cmd
}

// prepareManifest pulls or loads manifest to a target path and returns the local path
func prepareManifest(cmd *cobra.Command, url, file, outputDir string) (string, error) {
	if url != "" {
		cmd.Printf("🔗 Pulling manifest from URL: %s\n", url)
		if err := pullManifestWithORAS(url, outputDir); err != nil {
			return "", fmt.Errorf("failed to pull manifest: %v", err)
		}
		return findManifestFile(outputDir)
	}
	cmd.Printf("📄 Using local manifest: %s\n", file)
	return file, nil
}

// processAndPullArtifacts handles display, validation, and actual pull
func processAndPullArtifacts(cmd *cobra.Command, manifest *utils.ArtifactManifest, outputDir string) error {
	displayManifestInfo(cmd, manifest)
	displayArtifactSummary(cmd, manifest)

	total := len(manifest.Images) + len(manifest.Models) + len(manifest.Charts)
	if total == 0 {
		return fmt.Errorf("no artifacts found in manifest")
	}

	registry := extractRegistryFromManifest(manifest)
	if registry != "" {
		utils.CheckHarborLogin(registry)
	}

	if err := utils.PullArtifacts(manifest, outputDir); err != nil {
		return fmt.Errorf("failed to pull artifacts: %v", err)
	}

	cmd.Printf("✅ Pulled %d artifacts to %s\n", total, outputDir)
	return nil
}

// pullManifestWithORAS pulls a manifest file using ORAS
func pullManifestWithORAS(url, outputDir string) error {
	orasCmd := exec.Command("oras", "pull", url, "-o", outputDir)
	orasCmd.Stdout = os.Stdout
	orasCmd.Stderr = os.Stderr

	return orasCmd.Run()
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

func tagLocalResourcesAndPush(cmd *cobra.Command, manifest *utils.ArtifactManifest, localDir, targetRegistry string) error {
	cmd.Println("\n🚀 Tagging and pushing artifacts to:", targetRegistry)

	totalImages := len(manifest.Images)
	for i, image := range manifest.Images {
		if err := utils.RetagAndPushImage(i+1, totalImages, image, localDir, targetRegistry); err != nil {
			return fmt.Errorf("failed to push image %s: %w", image, err)
		}
	}
	totalModels := len(manifest.Models)
	for i, model := range manifest.Models {
		if err := utils.RetagAndPushModel(i+1, totalModels, model, localDir, targetRegistry); err != nil {
			return fmt.Errorf("failed to push model %s: %w", model, err)
		}
	}
	totalCharts := len(manifest.Charts)
	for i, chart := range manifest.Charts {
		if err := utils.RetagAndPushChart(i+1, totalCharts, chart, localDir, targetRegistry); err != nil {
			return fmt.Errorf("failed to push chart %s: %w", chart.HarborPath, err)
		}
	}

	cmd.Println("✅ All artifacts pushed successfully!")
	return nil
}

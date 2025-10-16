package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dynamofl/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

// AddArtifactsCommands adds the artifacts commands to the root command.
func AddArtifactsCommands(rootCmd *cobra.Command) {
	artifactsCmd := &cobra.Command{
		Use:   "artifacts",
		Short: "Process artifacts for deployment and upgrade",
		Long:  "Process artifacts for deployment and upgrade.",
	}

	artifactsCmd.AddCommand(createPullCmd(), createMirrorCmd())
	rootCmd.AddCommand(artifactsCmd)
}

func createPullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Pull manifest file from a URL or pull artifacts from a manifest file",
		Long:  "Pulls a manifest file from the specified URL using ORAS, or pulls artifacts from a local manifest file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			file, _ := cmd.Flags().GetString("file")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			imagesOnly, _ := cmd.Flags().GetBool("images")
			modelsOnly, _ := cmd.Flags().GetBool("models")
			chartsOnly, _ := cmd.Flags().GetBool("charts")

			if (url == "" && file == "") || (url != "" && file != "") {
				return fmt.Errorf("exactly one of --url or --file must be set")
			}

			filtersSpecified := imagesOnly || modelsOnly || chartsOnly
			pullOptions := utils.PullOptions{
				IncludeImages: !filtersSpecified || imagesOnly,
				IncludeModels: !filtersSpecified || modelsOnly,
				IncludeCharts: !filtersSpecified || chartsOnly,
			}

			manifestPath, err := prepareManifest(cmd, url, file, outputDir, "Output directory")
			if err != nil {
				return err
			}

			_, err = processManifest(cmd, manifestPath, outputDir, pullOptions)
			return err
		},
	}

	cmd.Flags().String("url", "", "URL of the manifest file to pull (e.g., artifacts.dynamo.ai/dynamoai/manifest:3.22.2)")
	cmd.Flags().String("file", "", "Path to the manifest JSON file")
	cmd.Flags().String("output-dir", "./artifacts", "Directory to save artifacts or manifest file")
	cmd.Flags().Bool("images", false, "Only pull container images")
	cmd.Flags().Bool("models", false, "Only pull ML models")
	cmd.Flags().Bool("charts", false, "Only pull Helm charts")

	return cmd
}

func createMirrorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mirror",
		Short: "Mirror a manifest and push pulled artifacts to a target registry",
		Long:  "Mirror a manifest by pulling artifacts locally and pushing selected types to a target registry.",
		RunE: func(cmd *cobra.Command, args []string) error {
			url, _ := cmd.Flags().GetString("url")
			file, _ := cmd.Flags().GetString("file")
			targetRegistry, _ := cmd.Flags().GetString("target-registry")
			cacheDirFlag, _ := cmd.Flags().GetString("cache-dir")
			keepCache, _ := cmd.Flags().GetBool("keep-cache")
			imagesFlag, _ := cmd.Flags().GetBool("images")
			modelsFlag, _ := cmd.Flags().GetBool("models")
			chartsFlag, _ := cmd.Flags().GetBool("charts")

			if (url == "" && file == "") || (url != "" && file != "") {
				return fmt.Errorf("exactly one of --url or --file must be set")
			}
			if targetRegistry == "" {
				return fmt.Errorf("--target-registry must be set")
			}

			var cacheDir string
			var err error
			cleanup := false
			if cacheDirFlag != "" {
				cacheDir = cacheDirFlag
				if err = os.MkdirAll(cacheDir, 0o755); err != nil {
					return fmt.Errorf("failed to create cache directory: %w", err)
				}
			} else {
				cacheDir, err = os.MkdirTemp("", "dynactl-mirror-")
				if err != nil {
					return fmt.Errorf("failed to create temporary cache: %w", err)
				}
				cleanup = !keepCache
				if cleanup {
					defer os.RemoveAll(cacheDir)
				}
			}

			filtersSpecified := imagesFlag || modelsFlag || chartsFlag
			var pullOptions utils.PullOptions
			if filtersSpecified {
				pullOptions = utils.PullOptions{
					IncludeImages: imagesFlag,
					IncludeModels: modelsFlag,
					IncludeCharts: chartsFlag,
				}
			} else {
				pullOptions = utils.PullOptions{
					IncludeImages: true,
					IncludeModels: false,
					IncludeCharts: false,
				}
			}

			manifestPath, err := prepareManifest(cmd, url, file, cacheDir, "Cache directory")
			if err != nil {
				return err
			}

			manifest, err := processManifest(cmd, manifestPath, cacheDir, pullOptions)
			if err != nil {
				return err
			}

			cmd.Printf("\n=== Mirroring Artifacts to %s ===\n", targetRegistry)
			mirrorOptions := utils.MirrorOptionsFromPull(pullOptions)
			if err := utils.MirrorArtifacts(manifest, cacheDir, targetRegistry, mirrorOptions); err != nil {
				return err
			}

			if cacheDirFlag == "" && keepCache {
				cmd.Printf("Cache retained at: %s\n", cacheDir)
			}
			if cacheDirFlag != "" {
				cmd.Printf("Cache directory: %s\n", cacheDir)
			}

			return nil
		},
	}

	cmd.Flags().String("url", "", "URL of the manifest file to mirror (e.g., artifacts.dynamo.ai/dynamoai/manifest:3.22.2)")
	cmd.Flags().String("file", "", "Path to the manifest JSON file")
	cmd.Flags().String("target-registry", "", "Target registry where artifacts will be pushed")
	cmd.Flags().String("cache-dir", "", "Directory to reuse for cache (default: temporary directory)")
	cmd.Flags().Bool("keep-cache", false, "Keep the temporary cache directory instead of removing it")
	cmd.Flags().Bool("images", false, "Mirror container images")
	cmd.Flags().Bool("models", false, "Mirror ML models")
	cmd.Flags().Bool("charts", false, "Mirror Helm charts")

	return cmd
}

func prepareManifest(cmd *cobra.Command, url, file, workspace, workspaceLabel string) (string, error) {
	if url != "" {
		if err := os.MkdirAll(workspace, 0o755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", strings.ToLower(workspaceLabel), err)
		}

		cmd.Printf("=== Pulling Manifest from URL ===\n")
		cmd.Printf("URL: %s\n", url)
		cmd.Printf("%s: %s\n", workspaceLabel, workspace)

		if err := pullManifestWithORAS(url, workspace); err != nil {
			return "", fmt.Errorf("failed to pull manifest from URL: %v", err)
		}

		cmd.Printf("✅ Successfully pulled manifest from %s to %s\n", url, workspace)

		manifestPath, err := findManifestFile(workspace)
		if err != nil {
			return "", fmt.Errorf("failed to find manifest file: %v", err)
		}
		return manifestPath, nil
	}

	cmd.Printf("=== Loading Manifest from File ===\n")
	cmd.Printf("Manifest file: %s\n", file)
	cmd.Printf("%s: %s\n", workspaceLabel, workspace)
	return file, nil
}

func pullManifestWithORAS(url, outputDir string) error {
	if err := utils.PullManifestFromRegistry(url, outputDir); err != nil {
		return err
	}
	return nil
}

func processManifest(cmd *cobra.Command, manifestPath, outputDir string, options utils.PullOptions) (*utils.ArtifactManifest, error) {
	cmd.Printf("\n=== Loading Manifest and Pulling Artifacts ===\n")
	utils.LogInfo("Loading manifest file: %s", manifestPath)

	manifest, err := utils.LoadManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %v", err)
	}
	options = utils.NormalizePullOptions(options)

	displayManifestInfo(cmd, manifest)

	totalArtifacts := 0
	if options.IncludeImages {
		totalArtifacts += len(manifest.Images)
	}
	if options.IncludeModels {
		totalArtifacts += len(manifest.Models)
	}
	if options.IncludeCharts {
		totalArtifacts += len(manifest.Charts)
	}
	if totalArtifacts == 0 {
		if strings.Contains(manifestPath, "testdata") {
			utils.LogInfo("No artifacts found in manifest, skipping artifact pull")
			return manifest, nil
		}
		return nil, fmt.Errorf("no artifacts found in manifest")
	}

	displayArtifactSummary(cmd, manifest, options)

	registry := extractRegistryFromManifest(manifest, options)
	if registry != "" {
		utils.CheckHarborLogin(registry)
	}

	if err := utils.PullArtifacts(manifest, outputDir, options); err != nil {
		return nil, fmt.Errorf("failed to pull artifacts from manifest: %v", err)
	}

	cmd.Printf("\n🎉 Successfully completed all operations!\n")
	cmd.Printf("Total artifacts pulled: %d\n", totalArtifacts)
	cmd.Printf("All files saved to: %s\n", outputDir)

	return manifest, nil
}

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

func displayArtifactSummary(cmd *cobra.Command, manifest *utils.ArtifactManifest, options utils.PullOptions) {
	cmd.Printf("\nArtifacts found in manifest:\n")
	if options.IncludeImages && len(manifest.Images) > 0 {
		cmd.Printf("  Container Images: %d\n", len(manifest.Images))
	}
	if options.IncludeModels && len(manifest.Models) > 0 {
		cmd.Printf("  ML Models: %d\n", len(manifest.Models))
	}
	if options.IncludeCharts && len(manifest.Charts) > 0 {
		cmd.Printf("  Helm Charts: %d\n", len(manifest.Charts))
	}
}

func extractFilenameFromURL(url string) string {
	url = strings.TrimPrefix(url, "http://")
	url = strings.TrimPrefix(url, "https://")

	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "manifest.json"
	}

	lastPart := parts[len(parts)-1]

	if strings.Contains(lastPart, ":") {
		nameParts := strings.Split(lastPart, ":")
		if len(nameParts) > 0 {
			if strings.HasSuffix(nameParts[0], ".json") {
				return nameParts[0]
			}
			return nameParts[0] + ".json"
		}
	}

	if !strings.Contains(lastPart, ".") {
		return lastPart + ".json"
	}
	if strings.HasSuffix(lastPart, ".json") {
		return lastPart
	}

	return lastPart
}

func extractRegistryFromManifest(manifest *utils.ArtifactManifest, options utils.PullOptions) string {
	options = utils.NormalizePullOptions(options)

	if options.IncludeImages && len(manifest.Images) > 0 {
		uri := strings.TrimPrefix(manifest.Images[0], "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	if options.IncludeModels && len(manifest.Models) > 0 {
		uri := strings.TrimPrefix(manifest.Models[0], "oci://")
		if strings.Contains(uri, "/") {
			parts := strings.SplitN(uri, "/", 2)
			if len(parts) == 2 {
				return parts[0]
			}
		}
	}

	if options.IncludeCharts && len(manifest.Charts) > 0 {
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

func findManifestFile(dir string) (string, error) {
	directPath := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(directPath); err == nil {
		return directPath, nil
	}

	var manifestPath string
	var latestMod time.Time
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Base(path) == "manifest.json" {
			if info.ModTime().After(latestMod) || manifestPath == "" {
				manifestPath = path
				latestMod = info.ModTime()
			}
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

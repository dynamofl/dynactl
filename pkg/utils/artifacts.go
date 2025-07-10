package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// ArtifactManifest represents the structure of the manifest file
type ArtifactManifest struct {
	CustomerID         string      `json:"customer_id"`
	CustomerName       string      `json:"customer_name"`
	ReleaseVersion     string      `json:"release_version"`
	OnboardingDate     string      `json:"onboarding_date"`
	LicenseGeneratedAt *string     `json:"license_generated_at"`
	LicenseExpiry      *string     `json:"license_expiry"`
	MaxUsers           *int        `json:"max_users"`
	SPOC               SPOC        `json:"spoc"`
	Artifacts          Artifacts   `json:"artifacts"`
	Images             []string    `json:"images"` // Array of OCI URIs
	Models             []string    `json:"models"` // Array of OCI URIs
	Charts             []HelmChart `json:"charts"`
}

type HelmChart struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	AppVersion string `json:"appVersion,omitempty"`
	Filename   string `json:"filename"`
	HarborPath string `json:"harbor_path"`
	SHA256     string `json:"sha256,omitempty"`
	SizeBytes  int64  `json:"size_bytes,omitempty"`
}

// SPOC represents the Single Point of Contact
type SPOC struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Artifacts represents the root paths for different artifact types
type Artifacts struct {
	ChartsRoot string `json:"charts_root"`
	ImagesRoot string `json:"images_root"`
	ModelsRoot string `json:"models_root"`
}

// Chart represents a Helm chart with additional metadata
type Chart struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	AppVersion string `json:"appVersion"`
	Filename   string `json:"filename"`
	HarborPath string `json:"harbor_path"`
	SHA256     string `json:"sha256"`
	SizeBytes  int64  `json:"size_bytes"`
}

// Component represents a unified artifact component for processing
type Component struct {
	Name      string
	Type      string
	URI       string
	Tag       string
	Digest    string
	MediaType string
}

// PullResult represents the result of pulling artifacts
type PullResult struct {
	TotalArtifacts int
	SuccessCount   int
	FailedCount    int
	Duration       time.Duration
	Errors         []string
}

// LoadManifest loads and parses the manifest file
func LoadManifest(filename string) (*ArtifactManifest, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open manifest file: %v", err)
	}
	defer file.Close()

	var manifest ArtifactManifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest file: %v", err)
	}

	return &manifest, nil
}

// PullArtifacts pulls all artifacts specified in the manifest from Harbor
func PullArtifacts(manifest *ArtifactManifest, outputDir string) error {
	components := convertManifestToComponents(manifest)

	LogInfo("=== Starting Artifact Pull Process ===")
	LogInfo("Total artifacts to pull: %d", len(components))
	LogInfo("Output directory: %s", outputDir)

	// Display component breakdown
	displayComponentBreakdown(components)

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Pull all artifacts and collect results
	result := pullAllArtifacts(components, outputDir)

	// Display summary
	displayPullSummary(result)

	if result.FailedCount > 0 {
		return fmt.Errorf("failed to pull %d artifacts", result.FailedCount)
	}

	LogInfo("🎉 Successfully pulled all %d artifacts!", len(components))
	return nil
}

// PullArtifacts pulls all artifacts specified in the manifest from Harbor
// func PushArtifacts(manifest *ArtifactManifest, targetRegistry string) error {
// 	components := convertManifestToComponents(manifest)

// 	LogInfo("=== Starting Artifact Push Process ===")
// 	LogInfo("Total artifacts to push: %d", len(components))
// 	LogInfo("Target registry: %s", targetRegistry)

// 	// Display component breakdown
// 	displayComponentBreakdown(components)

// 	// Pull all artifacts and collect results
// 	result := pushAllArtifacts(components, targetRegistry)

// 	// Display summary
// 	displayPullSummary(result)

// 	if result.FailedCount > 0 {
// 		return fmt.Errorf("failed to push %d artifacts", result.FailedCount)
// 	}

// 	LogInfo("🎉 Successfully pushed all %d artifacts!", len(components))
// 	return nil
// }

// displayComponentBreakdown displays a breakdown of components by type
func displayComponentBreakdown(components []Component) {
	LogInfo("Components breakdown:")

	imageCount := 0
	modelCount := 0
	chartCount := 0
	for _, comp := range components {
		switch comp.Type {
		case "containerImage":
			imageCount++
		case "mlModel":
			modelCount++
		case "helmChart":
			chartCount++
		}
	}

	if imageCount > 0 {
		LogInfo("  - Container Images: %d", imageCount)
	}
	if modelCount > 0 {
		LogInfo("  - ML Models: %d", modelCount)
	}
	if chartCount > 0 {
		LogInfo("  - Helm Charts: %d", chartCount)
	}
}

// pullAllArtifacts pulls all artifacts and returns a summary
func pullAllArtifacts(components []Component, outputDir string) PullResult {
	startTime := time.Now()
	result := PullResult{
		TotalArtifacts: len(components),
		SuccessCount:   0,
		FailedCount:    0,
		Errors:         []string{},
	}

	for i, component := range components {
		displayArtifactHeader(i+1, len(components), component)
		artifactStartTime := time.Now()
		if err := pullSingleArtifact(component, outputDir); err != nil {
			LogError("❌ Failed to pull artifact %s: %v", component.Name, err)
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", component.Name, err))
		} else {
			artifactDuration := time.Since(artifactStartTime)
			LogInfo("✅ Successfully pulled %s in %v", component.Name, artifactDuration)
			result.SuccessCount++
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// func pushAllArtifacts(components []Component, targetRegistry string) PullResult {
// 	startTime := time.Now()
// 	result := PullResult{
// 		TotalArtifacts: len(components),
// 		SuccessCount:   0,
// 		FailedCount:    0,
// 		Errors:         []string{},
// 	}

// 	for i, component := range components {
// 		displayArtifactHeader(i+1, len(components), component)

// 		artifactStartTime := time.Now()
// 		if err := pushSingleArtifact(component, targetRegistry); err != nil {
// 			LogError("❌ Failed to push artifact %s: %v", component.Name, err)
// 			result.FailedCount++
// 			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", component.Name, err))
// 		} else {
// 			artifactDuration := time.Since(artifactStartTime)
// 			LogInfo("✅ Successfully pushed %s in %v", component.Name, artifactDuration)
// 			result.SuccessCount++
// 		}
// 	}

// 	result.Duration = time.Since(startTime)
// 	return result
// }

// displayArtifactHeader displays the header for each artifact being pulled
func displayArtifactHeader(current, total int, component Component) {
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Pulling artifact %d/%d: %s (%s)\n", current, total, component.Name, component.Type)
	fmt.Println("------------------------------------------------------------")
	LogInfo("")
	LogInfo("=== Pulling Artifact %d/%d ===", current, total)
	LogInfo("Name: %s", component.Name)
	LogInfo("Type: %s", component.Type)
	LogInfo("URI: %s", component.URI)
	if component.Tag != "" {
		LogInfo("Tag: %s", component.Tag)
	}
}

// displayPullSummary displays a summary of the pull operation
func displayPullSummary(result PullResult) {
	LogInfo("")
	LogInfo("=== Pull Summary ===")
	LogInfo("Total time: %v", result.Duration)
	LogInfo("Successful: %d", result.SuccessCount)
	LogInfo("Failed: %d", result.FailedCount)
}

// convertManifestToComponents converts the new manifest format to unified components
func convertManifestToComponents(manifest *ArtifactManifest) []Component {
	var components []Component

	// Convert images (array of OCI URIs) to components
	for _, imgURI := range manifest.Images {
		uri := strings.TrimPrefix(imgURI, "oci://")
		components = append(components, Component{
			Name:      extractNameFromURI(uri),
			Type:      "containerImage",
			URI:       uri,
			Tag:       "",
			MediaType: "application/vnd.oci.image.manifest.v1+json",
		})
	}

	// Convert models (array of OCI URIs) to components
	for _, modelURI := range manifest.Models {
		uri := strings.TrimPrefix(modelURI, "oci://")
		components = append(components, Component{
			Name:      extractNameFromURI(uri),
			Type:      "mlModel",
			URI:       uri,
			Tag:       "",
			MediaType: "application/vnd.dynamoai.model.v1+tar.gz",
		})
	}

	// Convert charts to components
	for _, chart := range manifest.Charts {
		uri := strings.TrimPrefix(chart.HarborPath, "oci://")
		components = append(components, Component{
			Name:      chart.Name,
			Type:      "helmChart",
			URI:       uri,
			Tag:       chart.Version,
			MediaType: "application/vnd.oci.image.manifest.v1+json",
		})
	}

	return components
}

// extractNameFromURI extracts the last part of the path as the name
func extractNameFromURI(uri string) string {
	// Remove tag if present
	parts := strings.Split(uri, ":")
	path := parts[0]
	pathParts := strings.Split(path, "/")
	if len(pathParts) > 0 {
		return pathParts[len(pathParts)-1]
	}
	return uri
}

// pullSingleArtifact pulls a single artifact from Harbor
func pullSingleArtifact(component Component, outputDir string) error {
	switch component.Type {
	case "containerImage":
		return pullContainerImage(component, outputDir)
	case "helmChart":
		return pullHelmChart(component, outputDir)
	default:
		return pullOrasArtifact(component, outputDir)
	}
}

// // pushSingleArtifact pushes a single artifact to target registry
// func pushSingleArtifact(component Component, targetRegistry string) error {
// 	switch component.Type {
// 	case "containerImage":
// 		return pushContainerImage(component, targetRegistry)
// 	case "helmChart":
// 		return pushHelmChart(component, targetRegistry)
// 	default:
// 		return pushOrasArtifact(component, targetRegistry)
// 	}
// }

// CheckHarborLogin checks if the user is logged into Harbor
func CheckHarborLogin(registry string) error {
	LogInfo("Checking Harbor login status for registry: %s", registry)
	LogInfo("If you have trouble pulling artifacts, run: oras login %s", registry)
	return nil
}

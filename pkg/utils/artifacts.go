package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ArtifactManifest represents the structure of the manifest file
type ArtifactManifest struct {
	CustomerID         string    `json:"customer_id"`
	CustomerName       string    `json:"customer_name"`
	ReleaseVersion     string    `json:"release_version"`
	OnboardingDate     string    `json:"onboarding_date"`
	LicenseGeneratedAt string    `json:"license_generated_at"`
	LicenseExpiry      string    `json:"license_expiry"`
	MaxUsers           int       `json:"max_users"`
	SPOC               SPOC      `json:"spoc"`
	Images             []Image   `json:"images"`
	Models             []Model   `json:"models"`
	Charts             []Chart   `json:"charts"`
}

// SPOC represents the Single Point of Contact
type SPOC struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Image represents a container image
type Image struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
	Path string `json:"path"`
}

// Model represents a machine learning model
type Model struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
	Path string `json:"path"`
}

// Chart represents a Helm chart
type Chart struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	AppVersion string `json:"appVersion"`
	Path       string `json:"path"`
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
	// Convert manifest to components
	components := convertManifestToComponents(manifest)
	
	LogInfo("Pulling %d artifacts to directory: %s", len(components), outputDir)

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	for i, component := range components {
		LogInfo("Pulling artifact %d/%d: %s (%s)", i+1, len(components), component.Name, component.Type)
		
		if err := pullSingleArtifact(component, outputDir); err != nil {
			return fmt.Errorf("failed to pull artifact %s: %v", component.Name, err)
		}
	}

	LogInfo("Successfully pulled all artifacts")
	return nil
}

// convertManifestToComponents converts the new manifest format to unified components
func convertManifestToComponents(manifest *ArtifactManifest) []Component {
	var components []Component

	// Convert images to components
	for _, img := range manifest.Images {
		uri := strings.TrimPrefix(img.Path, "oci://")
		components = append(components, Component{
			Name:      img.Name,
			Type:      "containerImage",
			URI:       uri,
			Tag:       img.Tag,
			MediaType: "application/vnd.oci.image.manifest.v1+json",
		})
	}

	// Convert models to components
	for _, model := range manifest.Models {
		components = append(components, Component{
			Name:      model.Name,
			Type:      "mlModel",
			URI:       model.Path,
			Tag:       model.Tag,
			MediaType: "application/vnd.dynamoai.model.v1+tar.gz",
		})
	}

	// Convert charts to components
	for _, chart := range manifest.Charts {
		uri := strings.TrimPrefix(chart.Path, "oci://")
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

// pullSingleArtifact pulls a single artifact from Harbor
func pullSingleArtifact(component Component, outputDir string) error {
	// Handle container images using Docker
	if component.Type == "containerImage" {
		return pullContainerImage(component)
	}
	
	// Handle Helm charts using helm pull
	if component.Type == "helmChart" {
		return pullHelmChart(component, outputDir)
	}
	
	// Handle other artifacts using ORAS
	return pullOrasArtifact(component, outputDir)
}

// pullContainerImage pulls a container image using Docker
func pullContainerImage(component Component) error {
	var dockerImage string
	
	// Use tag if available, otherwise use digest
	if component.Tag != "" {
		// Use tag format
		dockerImage = fmt.Sprintf("%s:%s", component.URI, component.Tag)
	} else if component.Digest != "" {
		// Use digest format
		dockerImage = fmt.Sprintf("%s@%s", component.URI, component.Digest)
	} else {
		return fmt.Errorf("neither tag nor digest specified for container image %s", component.Name)
	}
	
	LogInfo("Pulling container image: %s", dockerImage)
	
	// Execute docker pull command
	cmd := exec.Command("docker", "pull", dockerImage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull container image: %v", err)
	}
	
	LogInfo("Successfully pulled container image: %s", dockerImage)
	return nil
}

// pullHelmChart pulls a Helm chart using helm pull
func pullHelmChart(component Component, outputDir string) error {
	// Construct the helm pull command
	helmChart := fmt.Sprintf("oci://%s", component.URI)
	
	LogInfo("Pulling Helm chart: %s --version %s", helmChart, component.Tag)
	
	// Execute helm pull command
	cmd := exec.Command("helm", "pull", helmChart, "--version", component.Tag, "--destination", outputDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull Helm chart: %v", err)
	}
	
	LogInfo("Successfully pulled Helm chart: %s", helmChart)
	return nil
}

// pullOrasArtifact pulls a non-container artifact using ORAS
func pullOrasArtifact(component Component, outputDir string) error {
	uri := component.URI
	if !strings.Contains(uri, "/") {
		return fmt.Errorf("invalid URI format: %s", uri)
	}

	LogInfo("Pulling ORAS artifact from URI: %s", uri)

	var reference string
	if component.Tag != "" {
		reference = fmt.Sprintf("%s:%s", uri, component.Tag)
		LogInfo("Pulling artifact with tag: %s", component.Tag)
	} else if component.Digest != "" {
		reference = fmt.Sprintf("%s@%s", uri, component.Digest)
		LogInfo("Pulling artifact with digest: %s", component.Digest)
	} else {
		return fmt.Errorf("neither tag nor digest specified for ORAS artifact %s", component.Name)
	}

	// Create a descriptive filename for the artifact
	var artifactPath string
	if component.Tag != "" {
		artifactPath = fmt.Sprintf("%s-%s.tar", component.Name, component.Tag)
	} else {
		artifactPath = fmt.Sprintf("%s-%s.tar", component.Name, strings.TrimPrefix(component.Digest, "sha256:")[:12])
	}
	artifactFullPath := filepath.Join(outputDir, artifactPath)

	LogInfo("Running: oras pull %s -o %s", reference, artifactFullPath)
	cmd := exec.Command("oras", "pull", reference, "-o", artifactFullPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("oras CLI failed: %v", err)
	}

	LogInfo("Successfully pulled artifact to: %s", artifactFullPath)
	return nil
}

// CheckHarborLogin checks if the user is logged into Harbor
func CheckHarborLogin(registry string) error {
	LogInfo("Checking Harbor login status for registry: %s", registry)
	LogInfo("If you have trouble pulling artifacts, run: oras login %s", registry)
	return nil
} 
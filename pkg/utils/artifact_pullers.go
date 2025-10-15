package utils

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	oras "oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry/remote"
	oras_auth "oras.land/oras-go/v2/registry/remote/auth"
)

// pullContainerImage pulls a container image using go-containerregistry
func pullContainerImage(component Component, outputDir string) error {
	var reference string
	if component.Tag != "" {
		reference = fmt.Sprintf("%s:%s", component.URI, component.Tag)
	} else if component.Digest != "" {
		reference = fmt.Sprintf("%s@%s", component.URI, component.Digest)
	} else {
		// Use the full URI as the reference (it should already include the tag or digest)
		reference = component.URI
	}

	LogInfo("📦 Pulling container image...")
	LogInfo("  Reference: %s", reference)

	// Set environment variable to suppress macOS malloc logging warnings
	originalMalloc := os.Getenv("MallocStackLogging")
	os.Setenv("MallocStackLogging", "0")
	defer func() {
		if originalMalloc != "" {
			os.Setenv("MallocStackLogging", originalMalloc)
		} else {
			os.Unsetenv("MallocStackLogging")
		}
	}()

	ref, err := name.ParseReference(reference)
	if err != nil {
		return fmt.Errorf("failed to parse image reference: %v", err)
	}

	LogInfo("  Downloading image layers...")
	img, err := crane.Pull(reference)
	if err != nil {
		return fmt.Errorf("failed to pull container image: %v", err)
	}

	// Save the image as a tar file in the outputDir
	tarPath := filepath.Join(outputDir, fmt.Sprintf("%s.tar", component.Name))
	LogInfo("  Saving image to: %s", tarPath)

	if err := crane.Save(img, ref.String(), tarPath); err != nil {
		return fmt.Errorf("failed to save container image: %v", err)
	}

	// Get file size for progress reporting
	if fileInfo, err := os.Stat(tarPath); err == nil {
		sizeMB := float64(fileInfo.Size()) / (1024 * 1024)
		LogInfo("  Image saved: %.2f MB", sizeMB)
	}

	return nil
}

// pullHelmChart pulls a Helm chart using Helm Go library
func pullHelmChart(component Component, outputDir string) error {
	// Extract the chart name from the HarborPath
	// HarborPath format: "oci://artifacts.dynamo.ai/dynamoai/3.22.2/charts/dynamoai-base-1.1.2.tgz"
	// We need: "oci://artifacts.dynamo.ai/dynamoai/3.22.2/charts/dynamoai-base"

	// Remove the .tgz extension first
	basePath := strings.TrimSuffix(component.URI, ".tgz")

	dirPath := path.Dir(basePath)
	fileBase := path.Base(basePath)

	if strings.HasSuffix(fileBase, "-"+component.Tag) {
		fileBase = strings.TrimSuffix(fileBase, "-"+component.Tag)
	}

	repoPath := dirPath
	if dirPath == "." || dirPath == "" {
		repoPath = fileBase
	} else if path.Base(dirPath) != fileBase {
		repoPath = path.Join(dirPath, fileBase)
	}

	if repoPath == "" {
		return fmt.Errorf("invalid chart path: %s", component.URI)
	}

	chartRef := fmt.Sprintf("oci://%s", repoPath)

	LogInfo("📊 Pulling Helm chart...")
	LogInfo("  Chart: %s", chartRef)
	LogInfo("  Version: %s", component.Tag)
	LogInfo("  Downloading chart files...")

	settings := cli.New()
	chartDownloader := downloader.ChartDownloader{
		Out:     os.Stdout,
		Getters: getter.All(settings),
		Options: []getter.Option{
			getter.WithPassCredentialsAll(true),
		},
	}

	// Download the chart to outputDir
	_, _, err := chartDownloader.DownloadTo(chartRef, component.Tag, outputDir)
	if err != nil {
		return fmt.Errorf("failed to download Helm chart: %v", err)
	}

	// Check if the chart was downloaded and report its size
	expectedChartFile := filepath.Join(outputDir, fmt.Sprintf("%s-%s.tgz", component.Name, component.Tag))
	if fileInfo, err := os.Stat(expectedChartFile); err == nil {
		sizeMB := float64(fileInfo.Size()) / (1024 * 1024)
		LogInfo("  Chart downloaded: %.2f MB", sizeMB)
	}

	return nil
}

// pullOrasArtifact pulls a non-container artifact using ORAS Go library
func pullOrasArtifact(component Component, outputDir string) error {
	uri := component.URI
	if !strings.Contains(uri, "/") {
		return fmt.Errorf("invalid URI format: %s", uri)
	}

	LogInfo("📁 Pulling ORAS artifact...")
	LogInfo("  URI: %s", uri)

	// Split the URI into repository and reference (tag or digest)
	repoPart, refPart := splitRepositoryAndReference(uri)
	if refPart == "" {
		refPart = "latest"
	}

	LogInfo("  Repository: %s", repoPart)
	LogInfo("  Reference: %s", refPart)
	LogInfo("  Downloading artifact...")

	var artifactPath string
	if refPart != "" && refPart != "latest" {
		artifactPath = fmt.Sprintf("%s-%s.tar", component.Name, refPart)
	} else {
		artifactPath = fmt.Sprintf("%s.tar", component.Name)
	}
	artifactFullPath := filepath.Join(outputDir, artifactPath)

	store, err := file.New(artifactFullPath)
	if err != nil {
		return fmt.Errorf("failed to create file store: %v", err)
	}
	defer store.Close()

	repo, err := remote.NewRepository(repoPart)
	if err != nil {
		return fmt.Errorf("failed to create ORAS repository for '%s': %v", repoPart, err)
	}

	// Use credentials for authentication
	repo.Client = &oras_auth.Client{
		Credential: func(ctx context.Context, registry string) (oras_auth.Credential, error) {
			cred, err := resolveRegistryCredential(registry)
			if err != nil {
				return oras_auth.Credential{}, err
			}
			return cred, nil
		},
	}

	_, err = oras.Copy(context.Background(), repo, refPart, store, "", oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("failed to pull ORAS artifact from '%s:%s': %v", repoPart, refPart, err)
	}

	// Get file size for progress reporting
	if fileInfo, err := os.Stat(artifactFullPath); err == nil {
		sizeMB := float64(fileInfo.Size()) / (1024 * 1024)
		LogInfo("  Artifact downloaded: %.2f MB", sizeMB)
	}

	LogInfo("  Saved to: %s", artifactFullPath)
	return nil
}

// PullManifestFromRegistry pulls a manifest artifact into the specified directory using the ORAS Go SDK.
func PullManifestFromRegistry(reference, outputDir string) error {
	if reference == "" {
		return fmt.Errorf("manifest reference cannot be empty")
	}

	trimmedRef := strings.TrimPrefix(reference, "oci://")
	repoPart, refPart := splitRepositoryAndReference(trimmedRef)
	if repoPart == "" {
		return fmt.Errorf("invalid manifest reference: %s", reference)
	}
	if refPart == "" {
		refPart = "latest"
	}

	LogInfo("📄 Pulling manifest artifact...")
	LogInfo("  Repository: %s", repoPart)
	LogInfo("  Reference: %s", refPart)

	store, err := file.New(outputDir)
	if err != nil {
		return fmt.Errorf("failed to create manifest output store: %v", err)
	}
	defer store.Close()
	store.AllowPathTraversalOnWrite = true

	repo, err := remote.NewRepository(repoPart)
	if err != nil {
		return fmt.Errorf("failed to create ORAS repository for '%s': %v", repoPart, err)
	}

	repo.Client = &oras_auth.Client{
		Credential: func(ctx context.Context, registry string) (oras_auth.Credential, error) {
			cred, err := resolveRegistryCredential(registry)
			if err != nil {
				return oras_auth.Credential{}, err
			}
			return cred, nil
		},
	}

	if _, err := oras.Copy(context.Background(), repo, refPart, store, "", oras.DefaultCopyOptions); err != nil {
		return fmt.Errorf("failed to pull manifest from '%s:%s': %v", repoPart, refPart, err)
	}

	LogInfo("  Manifest files saved under: %s", outputDir)
	return nil
}

// splitRepositoryAndReference splits an OCI URI into repository and reference (tag or digest)
// e.g. "artifacts.dynamo.ai/dynamoai/models/foo:latest" -> ("artifacts.dynamo.ai/dynamoai/models/foo", "latest")
//
//	"artifacts.dynamo.ai/dynamoai/models/foo@sha256:abcd" -> ("artifacts.dynamo.ai/dynamoai/models/foo", "sha256:abcd")
func splitRepositoryAndReference(uri string) (repo, ref string) {
	if i := strings.LastIndex(uri, ":"); i != -1 && !strings.Contains(uri[i+1:], "/") {
		// Tag
		repo = uri[:i]
		ref = uri[i+1:]
		return
	}
	if i := strings.LastIndex(uri, "@"); i != -1 {
		// Digest
		repo = uri[:i]
		ref = uri[i+1:]
		return
	}
	// No tag or digest
	return uri, ""
}

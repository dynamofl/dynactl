package utils

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// MirrorArtifacts pushes selected artifacts from the local cache into a target registry.
// Currently only container images are supported.
func MirrorArtifacts(manifest *ArtifactManifest, cacheDir, targetRegistry string, options MirrorOptions) error {
	options = NormalizeMirrorOptions(options)
	targetRegistry = strings.TrimSuffix(strings.TrimSpace(targetRegistry), "/")
	if targetRegistry == "" {
		return fmt.Errorf("target registry cannot be empty")
	}

	keychain := NewDynactlKeychain()

	if options.IncludeModels && len(manifest.Models) > 0 {
		return fmt.Errorf("mirroring ML models is not supported yet; rerun with --images to mirror container images only")
	}
	if options.IncludeCharts && len(manifest.Charts) > 0 {
		return fmt.Errorf("mirroring Helm charts is not supported yet; rerun with --images to mirror container images only")
	}

	if options.IncludeImages && len(manifest.Images) > 0 {
		LogInfo("=== Mirroring Container Images ===")
		if err := mirrorContainerImages(manifest.Images, cacheDir, targetRegistry, keychain); err != nil {
			return err
		}
	} else {
		LogInfo("No container images selected for mirroring")
	}

	LogInfo("Mirror operation completed successfully")
	return nil
}

func mirrorContainerImages(images []string, cacheDir, targetRegistry string, keychain authn.Keychain) error {
	for idx, imageRef := range images {
		current := idx + 1
		total := len(images)

		componentRef := strings.TrimPrefix(imageRef, "oci://")
		repoPart, tagOrDigest := splitRepositoryAndReference(componentRef)
		if repoPart == "" {
			return fmt.Errorf("invalid image reference: %s", imageRef)
		}
		if tagOrDigest == "" {
			return fmt.Errorf("image reference missing tag or digest: %s", imageRef)
		}

		imageName := extractNameFromURI(componentRef)
		tarPath := filepath.Join(cacheDir, fmt.Sprintf("%s.tar", imageName))

		targetRepo := buildTargetRepository(targetRegistry, repoPart)
		targetRef := assembleTargetReference(targetRepo, tagOrDigest)

		LogInfo("📤 Pushing image %d/%d", current, total)
		LogInfo("  Source: %s", componentRef)
		LogInfo("  Target: %s", targetRef)

		if err := pushImageFromTar(tarPath, targetRef, keychain); err != nil {
			return err
		}

		LogInfo("✅ Pushed %s (%d/%d)", targetRef, current, total)
	}
	return nil
}

func pushImageFromTar(tarPath, targetRef string, keychain authn.Keychain) error {
	img, err := tarball.ImageFromPath(tarPath, nil)
	if err != nil {
		return fmt.Errorf("failed to read image archive %s: %w", tarPath, err)
	}

	if err := crane.Push(img, targetRef, crane.WithAuthFromKeychain(keychain)); err != nil {
		return fmt.Errorf("failed to push image to %s: %w", targetRef, err)
	}

	return nil
}

func buildTargetRepository(targetRegistry, originalRepo string) string {
	trimmedTarget := strings.TrimSuffix(targetRegistry, "/")

	// Remove the original registry hostname from the repository path while preserving hierarchy.
	remainder := ""
	if slash := strings.Index(originalRepo, "/"); slash != -1 {
		remainder = originalRepo[slash+1:]
	}

	if remainder == "" {
		return trimmedTarget
	}
	return fmt.Sprintf("%s/%s", trimmedTarget, remainder)
}

func assembleTargetReference(targetRepo, tagOrDigest string) string {
	if strings.HasPrefix(tagOrDigest, "sha256:") {
		return fmt.Sprintf("%s@%s", targetRepo, tagOrDigest)
	}
	return fmt.Sprintf("%s:%s", targetRepo, tagOrDigest)
}

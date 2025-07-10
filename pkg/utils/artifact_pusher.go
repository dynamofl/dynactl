package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func RetagAndPushImage(current, total int, image string, localDir string, targetRegistry string) error {
	// Ensure it's an OCI image
	if !strings.HasPrefix(image, "oci://") {
		return fmt.Errorf("invalid image URI (missing oci://): %s", image)
	}

	// Strip the oci:// prefix
	imagePath := strings.TrimPrefix(image, "oci://")

	// Extract last part: name:tag
	parts := strings.Split(imagePath, "/")
	if len(parts) < 1 {
		return fmt.Errorf("invalid image path: %s", imagePath)
	}
	last := parts[len(parts)-1] // e.g. guard-inference:dynamoai-3.22.2

	// Extract name and tag
	nameTag := strings.SplitN(last, ":", 2)
	if len(nameTag) != 2 {
		return fmt.Errorf("invalid image format (expected name:tag): %s", last)
	}
	imageName := nameTag[0]
	imageTag := nameTag[1]

	// Construct full path to the .tar file
	imageFile := filepath.Join(localDir, fmt.Sprintf("%s.tar", imageName))

	// Verify file exists
	if _, err := os.Stat(imageFile); os.IsNotExist(err) {
		return fmt.Errorf("image file not found: %s", imageFile)
	}
	// Detect AWS ECR
	isECR := strings.Contains(targetRegistry, "amazonaws.com")

	var target string
	if isECR {
		// Flat naming for ECR
		target = fmt.Sprintf("%s:%s-%s", targetRegistry, imageName, imageTag)
	} else {
		// Preserve full structure
		target = fmt.Sprintf("%s/images/%s:%s", targetRegistry, imageName, imageTag)
	}

	// // Construct target path
	// target := fmt.Sprintf("%s/%s:%s", targetRegistry, imageName, imageTag)
	// target := fmt.Sprintf("%s:%s", path.Join(targetRegistry, imageName), imageTag)

	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Pushing image %d/%d:  %s (containerImage)\n", current, total, imageName)
	fmt.Println("------------------------------------------------------------")

	// Push using ORAS
	cmd := exec.Command("oras", "push", "--disable-path-validation",
		target,
		"--artifact-type", "application/vnd.oci.image.layer.v1.tar+gzip",
		fmt.Sprintf("%s:application/tar+gzip", imageFile),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RetagAndPushChart(current, total int, chart HelmChart, localDir, targetRegistry string) error {
	// Build full path to the .tgz file
	chartPath := filepath.Join(localDir, chart.Filename)

	// Verify file exists
	if _, err := os.Stat(chartPath); os.IsNotExist(err) {
		return fmt.Errorf("chart file not found: %s", chartPath)
	}

	// Detect AWS ECR
	isECR := strings.Contains(targetRegistry, "amazonaws.com")

	var target string
	if isECR {
		// Flat naming for ECR
		target = fmt.Sprintf("%s:%s-%s", targetRegistry, chart.Name, chart.Version)
	} else {
		// Preserve full structure
		target = fmt.Sprintf("%s/%s:%s", targetRegistry, chart.Name, chart.Version)
	}

	// Construct ORAS target like: <registry>/<name>:<version>
	// target := fmt.Sprintf("%s/%s:%s", targetRegistry, chart.Name, chart.Version)

	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Pushing artifact %d/%d:  %s (helmChart)\n", current, total, chart.Name)
	fmt.Println("------------------------------------------------------------")

	cmd := exec.Command("oras", "push", "--disable-path-validation",
		target,
		"--artifact-type", "application/vnd.cncf.helm.chart.v1",
		fmt.Sprintf("%s:application/tar+gzip", chartPath),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RetagAndPushModel(current, total int, model string, localDir, targetRegistry string) error {
	// Ensure it’s an OCI path
	if !strings.HasPrefix(model, "oci://") {
		return fmt.Errorf("invalid model URI: %s", model)
	}

	// Strip the "oci://" prefix
	modelPath := strings.TrimPrefix(model, "oci://")

	// Extract model name and tag from the final part
	parts := strings.Split(modelPath, "/")
	if len(parts) == 0 {
		return fmt.Errorf("invalid model path: %s", modelPath)
	}
	lastPart := parts[len(parts)-1] // e.g., "dynamoai-ml_pii_piiredaction_based_version_9:latest"
	nameTag := strings.SplitN(lastPart, ":", 2)
	if len(nameTag) != 2 {
		return fmt.Errorf("model tag not found in: %s", lastPart)
	}
	modelName := nameTag[0]
	modelTag := nameTag[1]

	// Construct local model path (you can change extension as needed)
	localModelPath := filepath.Join(localDir, fmt.Sprintf("%s.tar", modelName)) // adjust if using .onnx etc.

	// Check that the file exists
	if _, err := os.Stat(localModelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found: %s", localModelPath)
	}

	// Detect AWS ECR
	isECR := strings.Contains(targetRegistry, "amazonaws.com")

	var target string
	if isECR {
		// Flat naming for ECR
		target = fmt.Sprintf("%s:%s-%s", targetRegistry, modelName, modelTag)
	} else {
		// Preserve full structure
		target = fmt.Sprintf("%s/%s:%s", targetRegistry, modelName, modelTag)
	}

	// Compose the target registry path
	// target := fmt.Sprintf("%s/%s:%s", targetRegistry, modelName, modelTag)

	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Pushing model %d/%d: %s:%s\n", current, total, modelName, modelTag)
	fmt.Println("------------------------------------------------------------")

	// Push using ORAS
	cmd := exec.Command("oras", "push", "--disable-path-validation",
		target,
		"--artifact-type", "application/vnd.unknown.model.layer.v1",
		fmt.Sprintf("%s:application/tar", localModelPath),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

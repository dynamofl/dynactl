package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestArtifactsCommands(t *testing.T) {
	rootCmd := &cobra.Command{}
	AddArtifactsCommands(rootCmd)

	// Test that artifacts command exists
	artifactsCmd := findSubcommand(rootCmd, "artifacts")
	assert.NotNil(t, artifactsCmd, "artifacts command should exist")

	// Test that both pull commands exist
	pullURLCmd := findSubcommand(artifactsCmd, "pull")
	assert.NotNil(t, pullURLCmd, "pull command should exist")

	// Test URL flag exists
	urlFlag := pullURLCmd.Flags().Lookup("url")
	assert.NotNil(t, urlFlag, "url flag should exist")

	// Test output-dir flag exists
	outputDirFlag := pullURLCmd.Flags().Lookup("output-dir")
	assert.NotNil(t, outputDirFlag, "output-dir flag should exist")
	assert.Equal(t, "./artifacts", outputDirFlag.DefValue, "output-dir should have default value")
}

func TestExtractFilenameFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{
			url:      "artifacts.dynamo.ai/dynamoai/manifest:3.22.2",
			expected: "manifest.json",
		},
		{
			url:      "registry.example.com/project/manifest:latest",
			expected: "manifest.json",
		},
		{
			url:      "registry.example.com/project/config.json:1.0.0",
			expected: "config.json",
		},
		{
			url:      "registry.example.com/project/manifest",
			expected: "manifest.json",
		},
		{
			url:      "http://registry.example.com/project/manifest:3.22.2",
			expected: "manifest.json",
		},
		{
			url:      "https://registry.example.com/project/manifest:3.22.2",
			expected: "manifest.json",
		},
	}

	for _, test := range tests {
		result := extractFilenameFromURL(test.url)
		assert.Equal(t, test.expected, result, "URL: %s", test.url)
	}
}

func TestArtifactsPullCommand(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "dynactl-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	rootCmd := &cobra.Command{}
	AddArtifactsCommands(rootCmd)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	// Test with neither flag set
	rootCmd.SetArgs([]string{"artifacts", "pull"})
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one of --url or --file must be set")

	buf.Reset()
	// Test with both flags set
	rootCmd.SetArgs([]string{"artifacts", "pull", "--url", "foo", "--file", "bar"})
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one of --url or --file must be set")

	buf.Reset()
	// Test with only --url (invalid URL)
	rootCmd.SetArgs([]string{"artifacts", "pull", "--url", "invalid-url", "--output-dir", tempDir})
	err = rootCmd.Execute()
	assert.True(t, err != nil || bytes.Contains(buf.Bytes(), []byte("failed to pull manifest")) || bytes.Contains(buf.Bytes(), []byte("oras")), "should error or print failure when oras command is not available")

	buf.Reset()
	// Create a test manifest file
	manifestContent := `{
		"customer_id": "test-customer",
		"customer_name": "Test Customer",
		"release_version": "1.0.0",
		"images": [],
		"models": [],
		"charts": []
	}`
	manifestFile := filepath.Join(tempDir, "test-manifest.json")
	err = os.WriteFile(manifestFile, []byte(manifestContent), 0644)
	assert.NoError(t, err)

	// Test with only --file (non-existent file)
	rootCmd.SetArgs([]string{"artifacts", "pull", "--file", "non-existent.json", "--output-dir", tempDir})
	err = rootCmd.Execute()
	assert.True(t, err != nil || bytes.Contains(buf.Bytes(), []byte("failed to load manifest")), "should error or print failure when file does not exist")

	buf.Reset()
	// Test with only --file (valid manifest, but no artifacts)
	rootCmd.SetArgs([]string{"artifacts", "pull", "--file", manifestFile, "--output-dir", tempDir})
	err = rootCmd.Execute()
	assert.True(t, err != nil || bytes.Contains(buf.Bytes(), []byte("no artifacts found in manifest")), "should error or print failure when no artifacts found in manifest")
}

// Helper function to find a subcommand by name
func findSubcommand(cmd *cobra.Command, name string) *cobra.Command {
	for _, subcmd := range cmd.Commands() {
		if subcmd.Name() == name {
			return subcmd
		}
	}
	return nil
} 
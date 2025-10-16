package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestMainCommand(t *testing.T) {
	// Create a buffer to capture output
	var stdout, stderr bytes.Buffer

	// Test version command
	cmd := createTestRootCommand()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--version"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute version command: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, version) {
		t.Errorf("Expected output to contain version '%s', got '%s'", version, output)
	}

	// Test help command
	stdout.Reset()
	stderr.Reset()
	cmd.SetArgs([]string{"--help"})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("Failed to execute help command: %v", err)
	}
	output = stdout.String()
	if !strings.Contains(output, "Dynamo AI Deployment Tool") && !strings.Contains(output, "A Go-based tool to manage customer's DevOps operations") {
		t.Errorf("Expected help output to contain a recognizable description, got '%s'", output)
	}

	// Test invalid command
	stdout.Reset()
	stderr.Reset()
	cmd.SetArgs([]string{"invalid-command"})
	err = cmd.Execute()
	if err == nil && !strings.Contains(stderr.String(), "unknown command") {
		t.Error("Expected error or error message for invalid command")
	}
}

// createRootCommand creates a root command for testing
func createTestRootCommand() *cobra.Command {
	cmd := newRootCommand()

	// Add a dummy subcommand to ensure Cobra errors on unknown commands
	dummy := &cobra.Command{
		Use:   "dummy",
		Short: "Dummy subcommand",
		Run:   func(cmd *cobra.Command, args []string) {},
	}
	cmd.AddCommand(dummy)

	return cmd
}

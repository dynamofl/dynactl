package commands

import (
	"github.com/dynamoai/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

// AddClusterCommands adds the cluster commands to the root command
func AddClusterCommands(rootCmd *cobra.Command) {
	clusterCmd := &cobra.Command{
		Use:   "cluster",
		Short: "Handle cluster status",
		Long:  "Handle cluster status for the deployment.",
	}

	// Check command
	checkCmd := &cobra.Command{
		Use:   "check [--namespace <namespace>]",
		Short: "Check the cluster status",
		Long: `Checks the cluster status for the deployment, including:
  - Kubernetes version compatibility
  - Available vCPU, memory resources (more than 32 vCPU, 128 GB memory)
  - Required RBAC permissions in the namespace
    - Create deployment
    - Create PVC
    - Create service
    - Create configmap
    - Create secret
  - Cluster RBAC permissions
    - Create CRD`,
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")

			// Create Kubernetes checker
			kc, err := utils.NewKubernetesChecker()
			if err != nil {
				cmd.Printf("✗ Failed to connect to Kubernetes cluster: %v\n", err)
				return err
			}

			cmd.Printf("Checking cluster status for namespace: %s\n", namespace)
			cmd.Println()

			// Check Kubernetes version
			version, err := kc.CheckKubernetesVersion()
			if err != nil {
				cmd.Printf("✗ Kubernetes version: %s\n", version)
			} else {
				cmd.Printf("✓ Kubernetes version: %s\n", version)
			}

			// Check resources
			resources, err := kc.CheckResources()
			if err != nil {
				cmd.Printf("✗ Available resources: %s\n", resources)
			} else {
				cmd.Printf("✓ Available resources: %s\n", resources)
			}

			// Check namespace RBAC permissions
			nsRBAC, err := kc.CheckNamespaceRBAC(namespace)
			if err != nil {
				cmd.Printf("✗ RBAC permissions: %s\n", nsRBAC)
			} else {
				cmd.Printf("✓ RBAC permissions: %s\n", nsRBAC)
			}

			// Check cluster RBAC permissions
			clusterRBAC, err := kc.CheckClusterRBAC()
			if err != nil {
				cmd.Printf("✗ Cluster RBAC permissions: %s\n", clusterRBAC)
			} else {
				cmd.Printf("✓ Cluster RBAC permissions: %s\n", clusterRBAC)
			}

			// Check storage capacity
			storage, err := kc.CheckStorageCapacity()
			if err != nil {
				cmd.Printf("! Warning: %s\n", storage)
			} else {
				cmd.Printf("✓ Storage capacity: %s\n", storage)
			}

			cmd.Println()
			if err != nil {
				cmd.Printf("Cluster check failed: %v\n", err)
				return err
			}

			cmd.Println("✓ Cluster check completed successfully")
			return nil
		},
	}

	// Add namespace flag
	checkCmd.Flags().String("namespace", "", "Namespace to check (will be created if it doesn't exist)")
	checkCmd.MarkFlagRequired("namespace")

	// Add commands to cluster group
	clusterCmd.AddCommand(checkCmd)

	// Add cluster group to root command
	rootCmd.AddCommand(clusterCmd)
} 
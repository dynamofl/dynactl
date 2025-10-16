package commands

import (
	"github.com/dynamofl/dynactl/pkg/utils"
	"github.com/spf13/cobra"
)

// AddClusterCommands adds the cluster commands to the root command
func AddClusterCommands(rootCmd *cobra.Command) {
	clusterCmd := &cobra.Command{
		Use:   "cluster",
		Short: "Handle cluster status",
		Long:  "Handle cluster status for the deployment.",
	}

	// 'all check' - comprehensive check, requires namespace
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Run all cluster checks",
		Long:  "Runs all available cluster checks: version, node resources, namespace permissions, cluster permissions, and storage.",
	}
	allCheckCmd := &cobra.Command{
		Use:   "check [--namespace <namespace>]",
		Short: "Run all cluster checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")

			kc, err := utils.NewKubernetesChecker()
			if err != nil {
				cmd.Printf("✗ Failed to connect to Kubernetes cluster: %v\n", err)
				return err
			}

			cmd.Printf("Running all cluster checks for namespace: %s\n", namespace)
			cmd.Println()

			// Version
			version, err := kc.CheckKubernetesVersion()
			if err != nil {
				cmd.Printf("✗ Kubernetes version: %s\n", version)
			} else {
				cmd.Printf("✓ Kubernetes version: %s\n", version)
			}

			// Nodes/resources
			resources, err := kc.CheckResources("table")
			if err != nil {
				cmd.Printf("✗ Node resources: %s\n", resources)
			} else {
				cmd.Printf("✓ Node resources: %s\n", resources)
			}

			// Namespace permissions
			nsRBAC, err := kc.CheckNamespaceRBAC(namespace)
			if err != nil {
				cmd.Printf("✗ Namespace permissions: %s\n", nsRBAC)
			} else {
				cmd.Printf("✓ Namespace permissions: %s\n", nsRBAC)
			}

			// Cluster permissions
			clusterRBAC, err := kc.CheckClusterRBAC()
			if err != nil {
				cmd.Printf("✗ Cluster permissions: %s\n", clusterRBAC)
			} else {
				cmd.Printf("✓ Cluster permissions: %s\n", clusterRBAC)
			}

			// Storage classes compatibility
			scCompat, err := kc.CheckStorageClassesCompatibility()
			if err != nil {
				cmd.Printf("! StorageClasses: %s\n", scCompat)
			} else {
				cmd.Printf("✓ StorageClasses: %s\n", scCompat)
			}

			// Storage capacity
			storage, err := kc.CheckStorageCapacity()
			if err != nil {
				cmd.Printf("! Storage capacity: %s\n", storage)
			} else {
				cmd.Printf("✓ Storage capacity: %s\n", storage)
			}

			cmd.Println()
			if err != nil {
				cmd.Printf("One or more checks reported issues\n")
				return err
			}
			cmd.Println("✓ All checks completed successfully")
			return nil
		},
	}
	allCheckCmd.Flags().StringP("namespace", "n", "", "Namespace to check permissions in")
	allCheckCmd.MarkFlagRequired("namespace")
	allCmd.AddCommand(allCheckCmd)

	// 'node check' - node status/resources, no namespace required
	nodeCmd := &cobra.Command{
		Use:   "node",
		Short: "Check node status",
		Long:  "Checks node readiness and aggregated CPU/memory resources.",
	}
	nodeCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "Check node status",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := utils.NewKubernetesChecker()
			if err != nil {
				cmd.Printf("✗ Failed to connect to Kubernetes cluster: %v\n", err)
				return err
			}

			outputFormat, _ := cmd.Flags().GetString("output")

			cmd.Println("Checking node resources...")
			resources, err := kc.CheckResources(outputFormat)
			if err != nil {
				cmd.Printf("✗ Node resources: %s\n", resources)
				return err
			}
			cmd.Printf("✓ Node resources: %s\n", resources)
			return nil
		},
	}
	nodeCheckCmd.Flags().StringP("output", "o", "table", "Output format: table or csv")
	nodeCmd.AddCommand(nodeCheckCmd)

	// 'permission check' - namespace and cluster RBAC, namespace required
	permCmd := &cobra.Command{
		Use:   "permission",
		Short: "Check permissions",
		Long:  "Checks namespace-level and cluster-level permissions.",
	}
	permCheckCmd := &cobra.Command{
		Use:   "check [--namespace <namespace>]",
		Short: "Check permissions in a namespace",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")
			kc, err := utils.NewKubernetesChecker()
			if err != nil {
				cmd.Printf("✗ Failed to connect to Kubernetes cluster: %v\n", err)
				return err
			}
			nsRBAC, err := kc.CheckNamespaceRBAC(namespace)
			if err != nil {
				cmd.Printf("✗ Namespace permissions: %s\n", nsRBAC)
				return err
			}
			cmd.Printf("✓ Namespace permissions: %s\n", nsRBAC)

			clusterRBAC, err := kc.CheckClusterRBAC()
			if err != nil {
				cmd.Printf("✗ Cluster permissions: %s\n", clusterRBAC)
				return err
			}
			cmd.Printf("✓ Cluster permissions: %s\n", clusterRBAC)
			return nil
		},
	}
	permCheckCmd.Flags().StringP("namespace", "n", "", "Namespace to check permissions in")
	permCheckCmd.MarkFlagRequired("namespace")
	permCmd.AddCommand(permCheckCmd)

	// 'storage check' - storage classes compatibility and capacity
	storageCmd := &cobra.Command{
		Use:   "storage",
		Short: "Check storage health",
		Long:  "Checks StorageClasses for database compatibility and storage capacity.",
	}
	storageCheckCmd := &cobra.Command{
		Use:   "check",
		Short: "Check storage classes and capacity",
		RunE: func(cmd *cobra.Command, args []string) error {
			kc, err := utils.NewKubernetesChecker()
			if err != nil {
				cmd.Printf("✗ Failed to connect to Kubernetes cluster: %v\n", err)
				return err
			}

			scCompat, err := kc.CheckStorageClassesCompatibility()
			if err != nil {
				cmd.Printf("! StorageClasses: %s\n", scCompat)
			} else {
				cmd.Printf("✓ StorageClasses: %s\n", scCompat)
			}

			storage, err := kc.CheckStorageCapacity()
			if err != nil {
				cmd.Printf("! Storage capacity: %s\n", storage)
				return err
			}
			cmd.Printf("✓ Storage capacity: %s\n", storage)
			return nil
		},
	}
	storageCmd.AddCommand(storageCheckCmd)

	// Add commands to cluster group
	clusterCmd.AddCommand(allCmd)
	clusterCmd.AddCommand(nodeCmd)
	clusterCmd.AddCommand(permCmd)
	clusterCmd.AddCommand(storageCmd)

	// Add cluster group to root command
	rootCmd.AddCommand(clusterCmd)
}

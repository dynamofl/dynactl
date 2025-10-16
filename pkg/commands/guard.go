package commands

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/dynamofl/dynactl/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"
)

// AddGuardCommands adds the guard commands to the root command
func AddGuardCommands(rootCmd *cobra.Command) {
	guardCmd := &cobra.Command{
		Use:   "guard",
		Short: "Guard-related utilities",
		Long:  "Security and resource visibility utilities for Dynamo Guard.",
	}

	modelsCmd := &cobra.Command{
		Use:   "models",
		Short: "Model-related utilities",
		Long:  "Utilities for working with model serving components.",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List deployments and their resource requests/limits",
		Long:  "Lists deployments in the given namespace with CPU, memory, and GPU requests/limits per container.",
		RunE: func(cmd *cobra.Command, args []string) error {
			namespace, _ := cmd.Flags().GetString("namespace")
			output, _ := cmd.Flags().GetString("output")

			kc, err := utils.NewKubernetesChecker()
			if err != nil {
				cmd.Printf("✗ Failed to connect to Kubernetes cluster: %v\n", err)
				return err
			}

			summaries, err := kc.ListDeploymentResourceSummaries(namespace)
			if err != nil {
				cmd.Printf("✗ Failed to list deployments: %v\n", err)
				return err
			}

			// Exclude specific deployments by name
			exclude := map[string]struct{}{
				"dynamoai-data-processing": {},
				"dynamoai-moderation":      {},
				"dynamoai-off-topic":       {},
			}

			filtered := make([]utils.DeploymentResourceSummary, 0, len(summaries))
			for _, s := range summaries {
				if _, skip := exclude[s.Name]; skip {
					continue
				}
				filtered = append(filtered, s)
			}

			if len(filtered) == 0 {
				if output == "json" {
					cmd.Println("[]")
					return nil
				}
				cmd.Printf("No deployments found in namespace %s\n", namespace)
				return nil
			}

			if output == "json" {
				data, err := json.MarshalIndent(filtered, "", "  ")
				if err != nil {
					cmd.Printf("✗ Failed to marshal JSON: %v\n", err)
					return err
				}
				cmd.Println(string(data))
				return nil
			}

			if output == "csv" {
				writer := csv.NewWriter(cmd.OutOrStdout())
				_ = writer.Write([]string{
					"namespace",
					"deployment",
					"pods",
					"requests_cpu",
					"requests_memory",
					"requests_gpu",
					"limits_cpu",
					"limits_memory",
					"limits_gpu",
				})
				var totalPods int64
				for _, d := range filtered {
					reqCPU, reqMem, reqGPU, limCPU, limMem, limGPU := aggregateContainerResources(d.Containers)
					totalPods += int64(d.Pods)
					_ = writer.Write([]string{
						namespace,
						d.Name,
						fmt.Sprintf("%d", d.Pods),
						reqCPU,
						reqMem,
						reqGPU,
						limCPU,
						limMem,
						limGPU,
					})
				}
				// Totals row (aggregate across deployments, multiplied by pods)
				totals := computeTotals(filtered)
				_ = writer.Write([]string{
					namespace,
					"TOTAL",
					fmt.Sprintf("%d", totalPods),
					formatCPUCores(totals.requestsCPUMilliCores),
					formatGi(totals.requestsMemoryBytes),
					fmt.Sprintf("%d", totals.requestsGPUs),
					formatCPUCores(totals.limitsCPUMilliCores),
					formatGi(totals.limitsMemoryBytes),
					fmt.Sprintf("%d", totals.limitsGPUs),
				})
				writer.Flush()
				if err := writer.Error(); err != nil {
					cmd.Printf("✗ Failed to write CSV: %v\n", err)
					return err
				}
				return nil
			}

			// Header
			cmd.Printf("Namespace: %s\n", namespace)
			cmd.Println("Deployment (pods)                             Requests (cpu/mem/gpu)         Limits (cpu/mem/gpu)")
			cmd.Println("----------------------------------------------------------------------------------------------")

			for _, d := range filtered {
				reqCPU, reqMem, reqGPU, limCPU, limMem, limGPU := aggregateContainerResources(d.Containers)
				label := fmt.Sprintf("%s (%d)", d.Name, d.Pods)
				cmd.Printf("%-40s %-28s %-28s\n",
					label,
					joinTriple(reqCPU, reqMem, reqGPU),
					joinTriple(limCPU, limMem, limGPU),
				)
			}

			// Totals across all deployments (requests and limits) accounting for pod replicas
			totals := computeTotals(filtered)
			cmd.Println("----------------------------------------------------------------------------------------------")
			cmd.Printf("%-40s %-28s %-28s\n",
				"TOTAL (all deployments)",
				joinTriple(formatCPUCores(totals.requestsCPUMilliCores), formatGi(totals.requestsMemoryBytes), fmt.Sprintf("%d", totals.requestsGPUs)),
				joinTriple(formatCPUCores(totals.limitsCPUMilliCores), formatGi(totals.limitsMemoryBytes), fmt.Sprintf("%d", totals.limitsGPUs)),
			)

			return nil
		},
	}

	listCmd.Flags().StringP("namespace", "n", "", "Kubernetes namespace")
	_ = listCmd.MarkFlagRequired("namespace")
	listCmd.Flags().StringP("output", "o", "table", "Output format: table, json, or csv")

	modelsCmd.AddCommand(listCmd)
	guardCmd.AddCommand(modelsCmd)
	rootCmd.AddCommand(guardCmd)
}

// joinTriple joins cpu/memory/gpu strings into a compact display
func joinTriple(cpu, mem, gpu string) string {
	if cpu == "" {
		cpu = "-"
	}
	if mem == "" {
		mem = "-"
	}
	if gpu == "" {
		gpu = "-"
	}
	return cpu + "/" + mem + "/" + gpu
}

// aggregateContainerResources sums the requests/limits for cpu, memory, and gpu across containers
func aggregateContainerResources(containers []utils.ContainerResourceSummary) (string, string, string, string, string, string) {
	var reqCPU, reqMem, reqGPU resource.Quantity
	var limCPU, limMem, limGPU resource.Quantity

	// Initialize to zero
	reqCPU.Add(resource.MustParse("0"))
	reqMem.Add(resource.MustParse("0"))
	reqGPU.Add(resource.MustParse("0"))
	limCPU.Add(resource.MustParse("0"))
	limMem.Add(resource.MustParse("0"))
	limGPU.Add(resource.MustParse("0"))

	for _, c := range containers {
		if c.RequestsCPU != "" {
			reqCPU.Add(resource.MustParse(c.RequestsCPU))
		}
		if c.RequestsMemory != "" {
			reqMem.Add(resource.MustParse(c.RequestsMemory))
		}
		if c.RequestsGPU != "" {
			reqGPU.Add(resource.MustParse(c.RequestsGPU))
		}
		if c.LimitsCPU != "" {
			limCPU.Add(resource.MustParse(c.LimitsCPU))
		}
		if c.LimitsMemory != "" {
			limMem.Add(resource.MustParse(c.LimitsMemory))
		}
		if c.LimitsGPU != "" {
			limGPU.Add(resource.MustParse(c.LimitsGPU))
		}
	}

	return reqCPU.String(), reqMem.String(), reqGPU.String(), limCPU.String(), limMem.String(), limGPU.String()
}

type totalsAccumulator struct {
	requestsCPUMilliCores int64
	requestsMemoryBytes   int64
	requestsGPUs          int64
	limitsCPUMilliCores   int64
	limitsMemoryBytes     int64
	limitsGPUs            int64
}

func computeTotals(deployments []utils.DeploymentResourceSummary) totalsAccumulator {
	var t totalsAccumulator
	for _, d := range deployments {
		pods := int64(d.Pods)
		// per-deployment sums
		var reqCPU, reqMem, reqGPU resource.Quantity
		var limCPU, limMem, limGPU resource.Quantity
		for _, c := range d.Containers {
			if c.RequestsCPU != "" {
				reqCPU.Add(resource.MustParse(c.RequestsCPU))
			}
			if c.RequestsMemory != "" {
				reqMem.Add(resource.MustParse(c.RequestsMemory))
			}
			if c.RequestsGPU != "" {
				reqGPU.Add(resource.MustParse(c.RequestsGPU))
			}
			if c.LimitsCPU != "" {
				limCPU.Add(resource.MustParse(c.LimitsCPU))
			}
			if c.LimitsMemory != "" {
				limMem.Add(resource.MustParse(c.LimitsMemory))
			}
			if c.LimitsGPU != "" {
				limGPU.Add(resource.MustParse(c.LimitsGPU))
			}
		}
		// Multiply by number of pods
		t.requestsCPUMilliCores += reqCPU.MilliValue() * pods
		t.requestsMemoryBytes += reqMem.Value() * pods
		t.requestsGPUs += reqGPU.Value() * pods
		t.limitsCPUMilliCores += limCPU.MilliValue() * pods
		t.limitsMemoryBytes += limMem.Value() * pods
		t.limitsGPUs += limGPU.Value() * pods
	}
	return t
}

func formatCPUCores(milli int64) string {
	// 1000m == 1 core
	if milli%1000 == 0 {
		return fmt.Sprintf("%d", milli/1000)
	}
	return fmt.Sprintf("%.3f", float64(milli)/1000.0)
}

func formatGi(bytes int64) string {
	const gi = 1024 * 1024 * 1024
	if bytes%gi == 0 {
		return fmt.Sprintf("%dGi", bytes/gi)
	}
	return fmt.Sprintf("%.2fGi", float64(bytes)/float64(gi))
}

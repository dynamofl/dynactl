package utils

import (
    "context"
    "fmt"
    "sort"
    "strings"

    authorizationv1 "k8s.io/api/authorization/v1"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
    "k8s.io/client-go/tools/clientcmd"
)

// KubernetesChecker handles Kubernetes cluster checks
type KubernetesChecker struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewKubernetesChecker creates a new Kubernetes checker
func NewKubernetesChecker() (*KubernetesChecker, error) {
	// Try to load in-cluster config first, then fall back to kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
        // Fall back to kubeconfig respecting KUBECONFIG and default loading rules
        loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
        kubeCfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
        config, err = kubeCfg.ClientConfig()
        if err != nil {
            return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
        }
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	return &KubernetesChecker{
		clientset: clientset,
		config:    config,
	}, nil
}

// CheckKubernetesVersion returns the Kubernetes cluster server version
func (kc *KubernetesChecker) CheckKubernetesVersion() (string, error) {
	version, err := kc.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %v", err)
	}
	return version.GitVersion, nil
}

// NodeResourceUsage holds resource usage information for a node
type NodeResourceUsage struct {
	Name           string
	CPURequests    float64
	CPULimits      float64
	MemoryRequests float64
	MemoryLimits   float64
	GPURequests    int64
	GPULimits      int64
	CPUAllocatable float64
	MemoryAllocatable float64
	GPUAllocatable int64
	CPURequestsPercent float64
	CPULimitsPercent float64
	MemoryRequestsPercent float64
	MemoryLimitsPercent float64
}

// GetNodeResourceUsage calculates resource usage percentages for a specific node
func (kc *KubernetesChecker) GetNodeResourceUsage(nodeName string) (*NodeResourceUsage, error) {
	// Get all pods in all namespaces to calculate resource usage
	pods, err := kc.clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for node %s: %v", nodeName, err)
	}

	// Get node information
	node, err := kc.clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %v", nodeName, err)
	}

	usage := &NodeResourceUsage{
		Name: nodeName,
	}

	// Get allocatable resources
	if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
		usage.CPUAllocatable = float64((&cpu).MilliValue()) / 1000.0
	}
	if memory, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
		usage.MemoryAllocatable = float64((&memory).Value()) / (1024.0 * 1024.0 * 1024.0)
	}
	if gpu, ok := node.Status.Allocatable[corev1.ResourceName("nvidia.com/gpu")]; ok {
		usage.GPUAllocatable = (&gpu).Value()
	}

	// Calculate resource usage from pods
	for _, pod := range pods.Items {
		// Skip pods that are not running or are being terminated
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		for _, container := range pod.Spec.Containers {
			// CPU requests and limits
			if req, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				usage.CPURequests += float64((&req).MilliValue()) / 1000.0
			}
			if lim, ok := container.Resources.Limits[corev1.ResourceCPU]; ok {
				usage.CPULimits += float64((&lim).MilliValue()) / 1000.0
			}

			// Memory requests and limits
			if req, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				usage.MemoryRequests += float64((&req).Value()) / (1024.0 * 1024.0 * 1024.0)
			}
			if lim, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
				usage.MemoryLimits += float64((&lim).Value()) / (1024.0 * 1024.0 * 1024.0)
			}

			// GPU requests and limits
			if req, ok := container.Resources.Requests[corev1.ResourceName("nvidia.com/gpu")]; ok {
				usage.GPURequests += (&req).Value()
			}
			if lim, ok := container.Resources.Limits[corev1.ResourceName("nvidia.com/gpu")]; ok {
				usage.GPULimits += (&lim).Value()
			}
		}
	}

	if usage.CPUAllocatable > 0 {
		usage.CPURequestsPercent = usage.CPURequests / usage.CPUAllocatable * 100
		usage.CPULimitsPercent = usage.CPULimits / usage.CPUAllocatable * 100
	}
	if usage.MemoryAllocatable > 0 {
		usage.MemoryRequestsPercent = usage.MemoryRequests / usage.MemoryAllocatable * 100
		usage.MemoryLimitsPercent = usage.MemoryLimits / usage.MemoryAllocatable * 100
	}

	return usage, nil
}

// CheckResources checks available CPU and memory resources
func (kc *KubernetesChecker) CheckResources(outputFormat string) (string, error) {
	nodes, err := kc.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %v", err)
	}

	var totalCPURequests, totalMemoryRequests float64
	var totalCPUCores, totalMemoryGB float64
	readyNodes := 0

	LogInfo("Checking resources on %d nodes...", len(nodes.Items))

	// Create a slice to hold node information for sorting
	type nodeInfo struct {
		node         *corev1.Node
		instanceType string
	}
	var nodeInfos []nodeInfo

	// Collect node information and instance types
	for _, node := range nodes.Items {
		instanceType := "unknown"
		if it, ok := node.Labels["node.kubernetes.io/instance-type"]; ok {
			instanceType = it
		} else if it, ok := node.Labels["beta.kubernetes.io/instance-type"]; ok {
			instanceType = it
		} else if it, ok := node.Labels["node.k8s.io/instance-type"]; ok {
			instanceType = it
		}
		
		nodeInfos = append(nodeInfos, nodeInfo{node: &node, instanceType: instanceType})
	}

	// Sort by instance type alphabetically
	sort.Slice(nodeInfos, func(i, j int) bool {
		return nodeInfos[i].instanceType < nodeInfos[j].instanceType
	})

	// Print header based on output format
	if outputFormat == "csv" {
		fmt.Printf("Name,Type,CPU_Capacity_Cores,Memory_Capaclity_GB,CPU_Requests_%%,CPU_Limits_%%,Memory_Requests_%%,Memory_Limits_%%,GPU_Alloc_Total\n")
	} else {
		// Print table header
		fmt.Printf("Name\t\t\t\tType\t\tCPU\tMem(GB)\tCPU\tCPU\tMem\tMem\tGPU\n")
		fmt.Printf("\t\t\t\t\t\tCapcty\tCapcty\t%%Req\t%%Limit\t%%Req\t%%Limit\tAlloc/Total\n")
		fmt.Printf("----------------------------------------------------------------------------------------------------------------\n")
	}

	for _, nodeInfo := range nodeInfos {
		node := nodeInfo.node
		instanceType := nodeInfo.instanceType
		// Check if node is ready
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}
		
		if !isReady {
			LogInfo("Skipping node '%s' - not ready", node.Name)
			continue
		}

		readyNodes++

		// Get resource usage percentages
		usage, err := kc.GetNodeResourceUsage(node.Name)
		if err != nil {
			LogInfo("Node '%s' - failed to get usage: %v", node.Name, err)
			continue
		}

		// Accumulate the converted values for accurate metrics
		totalCPUCores += usage.CPUAllocatable
		totalMemoryGB += usage.MemoryAllocatable
		totalCPURequests += usage.CPURequests
		totalMemoryRequests += usage.MemoryRequests

		// Format GPU info
		gpuInfo := ""
		if usage.GPUAllocatable > 0 {
			gpuInfo = fmt.Sprintf("%d/%d", usage.GPURequests, usage.GPUAllocatable)
		}

		// Print row based on output format
		if outputFormat == "csv" {
			fmt.Printf("%s,%s,%.2f,%.2f,%.1f,%.1f,%.1f,%.1f,%s\n", 
				node.Name, instanceType, usage.CPUAllocatable, usage.MemoryAllocatable, 
				usage.CPURequestsPercent, usage.CPULimitsPercent,usage.MemoryRequestsPercent, usage.MemoryLimitsPercent, gpuInfo)
		} else {
			// Print table row
			fmt.Printf("%s\t%s\t%.2f\t%.2f\t%.1f%%\t%.1f%%\t%.1f%%\t%.1f%%\t%s\n", 
				node.Name, instanceType, usage.CPUAllocatable, usage.MemoryAllocatable, 
				usage.CPURequestsPercent, usage.CPULimitsPercent, usage.MemoryRequestsPercent, usage.MemoryLimitsPercent, gpuInfo)
		}
	}

	LogInfo("Total ready nodes: %d", readyNodes)
	
	// Log the difference between total capacity and allocatable for debugging
	LogInfo("Resource totals - CPU: %d cores allocatable; Memory: %d GB allocatable", totalCPUCores, totalMemoryGB)

	// Calculate aggregated percentages based on allocatable resources (consistent with individual node percentages)
	// This ensures the cluster summary percentages match what users see when they manually calculate
	// using the individual node percentages shown in the table
	aggregatedCPUPercent := float64(0)
	aggregatedMemoryPercent := float64(0)
	
	if totalCPUCores > 0 {
		aggregatedCPUPercent = float64(totalCPURequests) / float64(totalCPUCores) * 100
	}
	if totalMemoryGB > 0 {
		aggregatedMemoryPercent = float64(totalMemoryRequests) / float64(totalMemoryGB) * 100
	}

	// Calculate available resources (allocatable minus what's already requested)
	availableCPUCores := totalCPUCores - totalCPURequests
	availableMemoryGB := totalMemoryGB - totalMemoryRequests

	fmt.Printf("\nCLUSTER SUMMARY:\n")
	fmt.Printf("CPU: %.1f cores available, %.1f cores allocatable (%.1f%% already requested)\n", availableCPUCores, totalCPUCores, aggregatedCPUPercent)
	fmt.Printf("Mem: %.1f GB available, %.1f GB allocatable (%.1f%% already requested)\n", availableMemoryGB, totalMemoryGB, aggregatedMemoryPercent)

	return fmt.Sprintf("CPU: %.1f cores available, %.1f cores allocatable (%.1f%% already requested), Mem: %.1f GB available, %.1f GB allocatable (%.1f%% already requested)", 
		availableCPUCores, totalCPUCores, aggregatedCPUPercent, availableMemoryGB, totalMemoryGB, aggregatedMemoryPercent), nil
}

// CheckNamespaceRBAC checks RBAC permissions in the specified namespace using SelfSubjectAccessReview
func (kc *KubernetesChecker) CheckNamespaceRBAC(namespace string) (string, error) {
    type nsPerm struct {
        description string
        group       string
        resource    string
        verb        string
    }

    checks := []nsPerm{
        {description: "deployment create", group: "apps", resource: "deployments", verb: "create"},
        {description: "pvc create", group: "", resource: "persistentvolumeclaims", verb: "create"},
        {description: "service create", group: "", resource: "services", verb: "create"},
        {description: "configmap create", group: "", resource: "configmaps", verb: "create"},
        {description: "secret create", group: "", resource: "secrets", verb: "create"},
    }

    for _, c := range checks {
        LogInfo("Checking permission: %s in namespace '%s'...", c.description, namespace)
        ssar := &authorizationv1.SelfSubjectAccessReview{
            Spec: authorizationv1.SelfSubjectAccessReviewSpec{
                ResourceAttributes: &authorizationv1.ResourceAttributes{
                    Namespace: namespace,
                    Group:     c.group,
                    Resource:  c.resource,
                    Verb:      c.verb,
                },
            },
        }

        resp, err := kc.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(context.Background(), ssar, metav1.CreateOptions{})
        if err != nil {
            return "", fmt.Errorf("failed to perform access review for %s: %v", c.description, err)
        }
        if !resp.Status.Allowed {
            return "", fmt.Errorf("missing permission: %s in namespace %s (%s)", c.description, namespace, resp.Status.Reason)
        }
    }

    return "all required permissions available", nil
}

// CheckClusterRBAC checks cluster-level RBAC permissions using SelfSubjectAccessReview
func (kc *KubernetesChecker) CheckClusterRBAC() (string, error) {
    LogInfo("Checking cluster-level permission to create CRDs...")
    ssar := &authorizationv1.SelfSubjectAccessReview{
        Spec: authorizationv1.SelfSubjectAccessReviewSpec{
            ResourceAttributes: &authorizationv1.ResourceAttributes{
                Group:    "apiextensions.k8s.io",
                Resource: "customresourcedefinitions",
                Verb:     "create",
            },
        },
    }

    resp, err := kc.clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(context.Background(), ssar, metav1.CreateOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to perform cluster access review: %v", err)
    }
    if !resp.Status.Allowed {
        return "", fmt.Errorf("missing cluster permission to create CRDs (%s)", resp.Status.Reason)
    }

    return "all required cluster permissions available", nil
}

// CheckStorageCapacity checks available storage capacity
func (kc *KubernetesChecker) CheckStorageCapacity() (string, error) {
	pvs, err := kc.clientset.CoreV1().PersistentVolumes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list persistent volumes: %v", err)
	}

	var totalCapacity, usedCapacity int64
	for _, pv := range pvs.Items {
		if pv.Status.Phase == corev1.VolumeBound {
			capacity := pv.Spec.Capacity[corev1.ResourceStorage]
			usedCapacity += (&capacity).Value()
		}
		capVal := pv.Spec.Capacity[corev1.ResourceStorage]
		totalCapacity += (&capVal).Value()
	}

	if totalCapacity == 0 {
		return "no storage configured", nil
	}

	usagePercent := float64(usedCapacity) / float64(totalCapacity) * 100
	if usagePercent > 80 {
		return fmt.Sprintf("limited storage capacity (%.1f%% used)", usagePercent), 
			fmt.Errorf("storage usage above 80%%")
	}

	return fmt.Sprintf("adequate storage capacity (%.1f%% used)", usagePercent), nil
}

// ListNodeInstanceTypes returns a mapping of node name to instance type label
func (kc *KubernetesChecker) ListNodeInstanceTypes() (map[string]string, error) {
    nodes, err := kc.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list nodes: %v", err)
    }

    result := make(map[string]string, len(nodes.Items))
    for _, node := range nodes.Items {
        labels := node.Labels
        instanceType := labels["node.kubernetes.io/instance-type"]
        if instanceType == "" {
            instanceType = labels["beta.kubernetes.io/instance-type"]
        }
        if instanceType == "" {
            instanceType = labels["node.k8s.io/instance-type"]
        }
        if instanceType == "" {
            instanceType = "unknown"
        }
        result[node.Name] = instanceType
    }
    return result, nil
}

// CheckStorageClassesCompatibility checks StorageClasses for common database compatibility
func (kc *KubernetesChecker) CheckStorageClassesCompatibility() (string, error) {
    LogInfo("Checking StorageClasses for database compatibility...")
    storageClasses, err := kc.clientset.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
    if err != nil {
        return "", fmt.Errorf("failed to list StorageClasses: %v", err)
    }

    compatibleStorageClasses := []string{}
    for _, sc := range storageClasses.Items {
        provisioner := sc.Provisioner
        if strings.Contains(provisioner, "ebs") || // AWS EBS
            strings.Contains(provisioner, "azure") || // Azure Disk
            strings.Contains(provisioner, "gce") || // GCP PD
            strings.Contains(provisioner, "csi") || // CSI drivers
            strings.Contains(provisioner, "nfs") || // NFS
            strings.Contains(provisioner, "iscsi") || // iSCSI
            strings.Contains(provisioner, "local") { // Local storage
            compatibleStorageClasses = append(compatibleStorageClasses, sc.Name)
            LogInfo("Found compatible StorageClass '%s' with provisioner '%s'", sc.Name, provisioner)
        }
    }

    if len(compatibleStorageClasses) == 0 {
        return "no compatible StorageClasses found for common databases", fmt.Errorf("no compatible StorageClasses")
    }

    return fmt.Sprintf("compatible StorageClasses: %s", strings.Join(compatibleStorageClasses, ", ")), nil
}

// ContainerResourceSummary holds resource info for a container
type ContainerResourceSummary struct {
    Name           string
    RequestsCPU    string
    RequestsMemory string
    RequestsGPU    string
    LimitsCPU      string
    LimitsMemory   string
    LimitsGPU      string
}

// DeploymentResourceSummary holds resource info for a deployment
type DeploymentResourceSummary struct {
    Name       string
    Pods       int32
    Containers []ContainerResourceSummary
}

// ListDeploymentResourceSummaries lists deployments and summarizes container resource requests/limits
func (kc *KubernetesChecker) ListDeploymentResourceSummaries(namespace string) ([]DeploymentResourceSummary, error) {
    deployments, err := kc.clientset.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to list deployments in %s: %v", namespace, err)
    }

    summaries := make([]DeploymentResourceSummary, 0, len(deployments.Items))

    for _, d := range deployments.Items {
        depSummary := DeploymentResourceSummary{
            Name:       d.Name,
            Pods:       d.Status.Replicas,
            Containers: make([]ContainerResourceSummary, 0, len(d.Spec.Template.Spec.Containers)),
        }
        for _, c := range d.Spec.Template.Spec.Containers {
            req := c.Resources.Requests
            lim := c.Resources.Limits

            // CPU
            var reqCPU, limCPU string
            if q, ok := req[corev1.ResourceCPU]; ok {
                reqCPU = q.String()
            }
            if q, ok := lim[corev1.ResourceCPU]; ok {
                limCPU = q.String()
            }

            // Memory
            var reqMem, limMem string
            if q, ok := req[corev1.ResourceMemory]; ok {
                reqMem = q.String()
            }
            if q, ok := lim[corev1.ResourceMemory]; ok {
                limMem = q.String()
            }

            // GPU (nvidia.com/gpu)
            var reqGPU, limGPU string
            gpuRes := corev1.ResourceName("nvidia.com/gpu")
            if q, ok := req[gpuRes]; ok {
                reqGPU = q.String()
            }
            if q, ok := lim[gpuRes]; ok {
                limGPU = q.String()
            }

            depSummary.Containers = append(depSummary.Containers, ContainerResourceSummary{
                Name:           c.Name,
                RequestsCPU:    reqCPU,
                RequestsMemory: reqMem,
                RequestsGPU:    reqGPU,
                LimitsCPU:      limCPU,
                LimitsMemory:   limMem,
                LimitsGPU:      limGPU,
            })
        }
        summaries = append(summaries, depSummary)
    }

    return summaries, nil
}
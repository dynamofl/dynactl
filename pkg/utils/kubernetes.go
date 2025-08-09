package utils

import (
    "context"
    "fmt"
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

// CheckResources checks available CPU and memory resources
func (kc *KubernetesChecker) CheckResources() (string, error) {
	nodes, err := kc.clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list nodes: %v", err)
	}

	var totalCPU, totalMemory, allocatableCPU, allocatableMemory int64
	readyNodes := 0

	LogInfo("Checking resources on %d nodes...", len(nodes.Items))

	for _, node := range nodes.Items {
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

		// Parse CPU and memory
		cpu := node.Status.Allocatable[corev1.ResourceCPU]
		memory := node.Status.Allocatable[corev1.ResourceMemory]

		allocatableCPU += (&cpu).MilliValue()
		allocatableMemory += (&memory).Value()

		// Get total capacity
		cpuCap := node.Status.Capacity[corev1.ResourceCPU]
		memCap := node.Status.Capacity[corev1.ResourceMemory]
		totalCPU += (&cpuCap).MilliValue()
		totalMemory += (&memCap).Value()

		// Convert to human readable format for this node
		nodeCpuCores := (&cpu).MilliValue() / 1000
		nodeCpuTotal := (&cpuCap).MilliValue() / 1000
		nodeMemoryGB := (&memory).Value() / (1024 * 1024 * 1024)
		nodeMemoryTotalGB := (&memCap).Value() / (1024 * 1024 * 1024)

		LogInfo("Node '%s': %d/%d CPU cores, %d/%d GB memory (allocatable/total)", 
			node.Name, nodeCpuCores, nodeCpuTotal, nodeMemoryGB, nodeMemoryTotalGB)
	}

	LogInfo("Total ready nodes: %d", readyNodes)

	// Convert to human readable format
	cpuCores := allocatableCPU / 1000
	memoryGB := allocatableMemory / (1024 * 1024 * 1024)

	LogInfo("Cluster total: %d/%d CPU cores, %d/%d GB memory (allocatable/total)", 
		cpuCores, totalCPU/1000, memoryGB, totalMemory/(1024*1024*1024))

	return fmt.Sprintf("%d/%d CPU cores, %d/%d GB memory (allocatable/total)", 
		cpuCores, totalCPU/1000, memoryGB, totalMemory/(1024*1024*1024)), nil
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
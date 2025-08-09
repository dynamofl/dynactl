package utils

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

// KubernetesChecker handles Kubernetes cluster checks
type KubernetesChecker struct {
	clientset *kubernetes.Clientset
	crdClient *apiextensionsclient.Clientset
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

	crdClient, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create apiextensions client: %v", err)
	}

	return &KubernetesChecker{
		clientset: clientset,
		crdClient: crdClient,
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

	// Check for compatible StorageClasses for MongoDB and PostgreSQL
	LogInfo("Checking StorageClasses for database compatibility...")
	storageClasses, err := kc.clientset.StorageV1().StorageClasses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		LogInfo("Warning: failed to list StorageClasses: %v", err)
	} else {
		compatibleStorageClasses := []string{}
		for _, sc := range storageClasses.Items {
			// Check for common database-compatible provisioners
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
		
		if len(compatibleStorageClasses) > 0 {
			LogInfo("Found %d compatible StorageClasses for databases: %v", len(compatibleStorageClasses), compatibleStorageClasses)
		} else {
			LogInfo("Warning: No compatible StorageClasses found for MongoDB/PostgreSQL")
		}
	}

	return fmt.Sprintf("%d/%d CPU cores, %d/%d GB memory (allocatable/total)", 
		cpuCores, totalCPU/1000, memoryGB, totalMemory/(1024*1024*1024)), nil
}

// CheckNamespaceRBAC checks RBAC permissions in the specified namespace
func (kc *KubernetesChecker) CheckNamespaceRBAC(namespace string) (string, error) {
	// Check if namespace exists, create if it doesn't
	_, err := kc.clientset.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		// Try to create the namespace
		LogInfo("Namespace '%s' does not exist. Creating...", namespace)
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		_, err = kc.clientset.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create namespace %s: %v", namespace, err)
		}
		LogInfo("Created namespace: %s", namespace)
	}

	// Test resource creation with cleanup
	testResources := []struct {
		name     string
		createFn func() error
		cleanupFn func() error
	}{
		{
			name: "deployment",
			createFn: func() error {
				LogInfo("Creating test deployment in namespace '%s'...", namespace)
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dynactl-test-deployment",
						Namespace: namespace,
					},
					Spec: appsv1.DeploymentSpec{
						Replicas: int32Ptr(0), // 0 replicas to avoid actual workload
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "dynactl-test"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "dynactl-test"},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name:  "test",
										Image: "busybox:latest",
										Command: []string{"sleep", "1"},
									},
								},
							},
						},
					},
				}
				_, err := kc.clientset.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
				return err
			},
			cleanupFn: func() error {
				LogInfo("Cleaning up test deployment in namespace '%s'...", namespace)
				return kc.clientset.AppsV1().Deployments(namespace).Delete(context.Background(), "dynactl-test-deployment", metav1.DeleteOptions{})
			},
		},
		{
			name: "persistentvolumeclaim",
			createFn: func() error {
				LogInfo("Creating test PVC in namespace '%s'...", namespace)
				pvc := &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dynactl-test-pvc",
						Namespace: namespace,
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Mi"), // Minimal size
							},
						},
					},
				}
				_, err := kc.clientset.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
				return err
			},
			cleanupFn: func() error {
				LogInfo("Cleaning up test PVC in namespace '%s'...", namespace)
				return kc.clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), "dynactl-test-pvc", metav1.DeleteOptions{})
			},
		},
		{
			name: "service",
			createFn: func() error {
				LogInfo("Creating test service in namespace '%s'...", namespace)
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dynactl-test-service",
						Namespace: namespace,
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Port: 80,
								TargetPort: intstr.FromInt(80),
							},
						},
						Selector: map[string]string{"app": "dynactl-test"},
					},
				}
				_, err := kc.clientset.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
				return err
			},
			cleanupFn: func() error {
				LogInfo("Cleaning up test service in namespace '%s'...", namespace)
				return kc.clientset.CoreV1().Services(namespace).Delete(context.Background(), "dynactl-test-service", metav1.DeleteOptions{})
			},
		},
		{
			name: "configmap",
			createFn: func() error {
				LogInfo("Creating test configmap in namespace '%s'...", namespace)
				configmap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dynactl-test-configmap",
						Namespace: namespace,
					},
					Data: map[string]string{
						"test": "value",
					},
				}
				_, err := kc.clientset.CoreV1().ConfigMaps(namespace).Create(context.Background(), configmap, metav1.CreateOptions{})
				return err
			},
			cleanupFn: func() error {
				LogInfo("Cleaning up test configmap in namespace '%s'...", namespace)
				return kc.clientset.CoreV1().ConfigMaps(namespace).Delete(context.Background(), "dynactl-test-configmap", metav1.DeleteOptions{})
			},
		},
		{
			name: "secret",
			createFn: func() error {
				LogInfo("Creating test secret in namespace '%s'...", namespace)
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dynactl-test-secret",
						Namespace: namespace,
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						"test": []byte("value"),
					},
				}
				_, err := kc.clientset.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{})
				return err
			},
			cleanupFn: func() error {
				LogInfo("Cleaning up test secret in namespace '%s'...", namespace)
				return kc.clientset.CoreV1().Secrets(namespace).Delete(context.Background(), "dynactl-test-secret", metav1.DeleteOptions{})
			},
		},
	}
	
	var cleanupErrors []string
	
	for _, resource := range testResources {
		// Try to create the test resource
		LogInfo("Validating create permission for %s...", resource.name)
		err := resource.createFn()
		if err != nil {
			return "", fmt.Errorf("missing create permission for %s in namespace %s: %v", resource.name, namespace, err)
		}
		
		// Clean up immediately after successful creation
		if err := resource.cleanupFn(); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("failed to cleanup %s: %v", resource.name, err))
		}
	}
	
	// Log cleanup errors but don't fail the check
	if len(cleanupErrors) > 0 {
		LogInfo("Cleanup warnings: %v", cleanupErrors)
	}

	return "all required permissions available", nil
}

// CheckClusterRBAC checks cluster-level RBAC permissions
func (kc *KubernetesChecker) CheckClusterRBAC() (string, error) {
	// Try to create a test CRD to check cluster-level permissions
	LogInfo("Testing CRD create permission...")

	LogInfo("Creating test CRD 'dynactltests.dynactl.io'...")
	testCRD := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "dynactltests.dynactl.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "dynactl.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     "DynactlTest",
				ListKind: "DynactlTestList",
				Plural:   "dynactltests",
				Singular: "dynactltest",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"test": {
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := kc.crdClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(), testCRD, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("missing permission to create CRDs: %v", err)
	}
	LogInfo("Successfully created test CRD 'dynactltests.dynactl.io'.")

	// Clean up the test CRD immediately
	LogInfo("Cleaning up test CRD 'dynactltests.dynactl.io'...")
	err = kc.crdClient.ApiextensionsV1().CustomResourceDefinitions().Delete(context.Background(), "dynactltests.dynactl.io", metav1.DeleteOptions{})
	if err != nil {
		LogInfo("Warning: failed to cleanup test CRD: %v", err)
	} else {
		LogInfo("Successfully cleaned up test CRD 'dynactltests.dynactl.io'.")
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

// int32Ptr returns a pointer to an int32
func int32Ptr(i int32) *int32 {
	return &i
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
package e2e

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
)

//go:embed testdata/connectivity-test-pod.yaml
var testData embed.FS

//go:embed testdata/custom-netpolicy.yaml
var testNetPolicyData embed.FS

const (
	operandNetworkPolicyName = "allow-all-egress-and-metrics-ingress-operand"
	operandAppLabelKey       = "app"
	operandLeaseName         = "cli-manager-lock"

	metricsPort int32 = 60000
	pluginPort  int32 = 9449
	healthPort  int32 = 8443
	unusedPort  int32 = 8080

	clusterMonitoringLabel = "openshift.io/cluster-monitoring"
	ingressPolicyGroupKey  = "policy-group.network.openshift.io/ingress"

	openshiftMonitoringNamespace  = "openshift-monitoring"
	openshiftUWMNamespace         = "openshift-user-workload-monitoring"
	openshiftDNSNamespace         = "openshift-dns"
	prometheusK8sServiceName      = "prometheus-k8s"
	connectivityTimeout           = 2 * time.Minute
	networkPolicyReconcileTimeout = 10 * time.Minute
)

// ServiceClusterIPs returns the primary ClusterIP for a service, or nil if the service has no ClusterIP.
func ServiceClusterIPs(svc *corev1.Service) []string {
	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return nil
	}
	return []string{svc.Spec.ClusterIP}
}

func namespaceExists(ctx context.Context, client kubernetes.Interface, namespace string) bool {
	_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	return err == nil
}

// runConnectivityCheck creates a test pod from embedded YAML manifest and verifies connectivity
// to the specified server IP and port. The pod is created with the given labels and configuration,
// then monitored for completion to determine if the connection attempt was successful.
func runConnectivityCheck(ctx context.Context, kubeClient kubernetes.Interface, namespace string, labels map[string]string, serverIP string, port int32, hostNetwork bool, nodeName string) (bool, error) {
	// Load pod definition from embedded YAML manifest
	podYAML, err := testData.ReadFile("testdata/connectivity-test-pod.yaml")
	if err != nil {
		return false, fmt.Errorf("failed to load embedded pod manifest: %w", err)
	}

	// Replace template variables in YAML
	podYAML = []byte(strings.ReplaceAll(string(podYAML), "{{NAMESPACE}}", namespace))
	podYAML = []byte(strings.ReplaceAll(string(podYAML), "{{TARGET}}", fmt.Sprintf("%s:%d", serverIP, port)))

	// Unmarshal YAML to Pod object
	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(podYAML, pod); err != nil {
		return false, fmt.Errorf("failed to unmarshal pod manifest: %w", err)
	}

	// Set dynamic fields
	pod.Labels = labels
	pod.Spec.NodeName = nodeName
	pod.Spec.HostNetwork = hostNetwork

	// Set DNS policy based on host network configuration
	if hostNetwork {
		pod.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}

	created, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	podName := created.Name
	defer func() {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = kubeClient.CoreV1().Pods(namespace).Delete(deleteCtx, podName, metav1.DeleteOptions{})
	}()

	if err := WaitForPodCompletion(ctx, kubeClient, namespace, podName); err != nil {
		return false, err
	}
	completed, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if len(completed.Status.ContainerStatuses) == 0 {
		return false, fmt.Errorf("no container status recorded for pod %s", podName)
	}
	terminated := completed.Status.ContainerStatuses[0].State.Terminated
	if terminated == nil {
		return false, fmt.Errorf("container in pod %s has not terminated", podName)
	}
	return terminated.ExitCode == 0, nil
}

// ExpectConnectivity checks connectivity from a pod in the given namespace
// (with clientLabels) to each serverIP on the specified port (IPv4 only).
func ExpectConnectivity(ctx context.Context, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool) {
	for _, ip := range serverIPs {
		g.By(fmt.Sprintf("checking IPv4 connectivity %s -> %s:%d expected=%t", namespace, ip, port, shouldSucceed))
		err := pollConnectivity(ctx, kubeClient, namespace, clientLabels, ip, port, shouldSucceed, false, "", connectivityTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("connectivity check failed for %s -> %s:%d (expected %t)", namespace, ip, port, shouldSucceed))
	}
}

// ExpectHostNetworkConnectivity checks connectivity from a host-network pod on
// the given node (IPv4 only). Kubelet / host-network traffic bypasses NetworkPolicy.
func ExpectHostNetworkConnectivity(ctx context.Context, kubeClient kubernetes.Interface, namespace, nodeName string, serverIPs []string, port int32, shouldSucceed bool) {
	for _, ip := range serverIPs {
		g.By(fmt.Sprintf("checking IPv4 host-network connectivity node=%s -> %s:%d expected=%t", nodeName, ip, port, shouldSucceed))
		err := pollConnectivity(ctx, kubeClient, namespace, nil, ip, port, shouldSucceed, true, nodeName, connectivityTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("host-network connectivity check failed for node=%s -> %s:%d (expected %t)", nodeName, ip, port, shouldSucceed))
	}
}

func pollConnectivity(ctx context.Context, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIP string, port int32, shouldSucceed, hostNetwork bool, nodeName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		succeeded, err := runConnectivityCheck(ctx, kubeClient, namespace, clientLabels, serverIP, port, hostNetwork, nodeName)
		if err != nil {
			return false, nil
		}
		return succeeded == shouldSucceed, nil
	})
}

// WaitForPodCompletion waits up to 2 minutes for a pod to reach terminal state (Succeeded or Failed).
// It's optimized for the specific use case of waiting for a single connectivity test pod to complete.
func WaitForPodCompletion(ctx context.Context, kubeClient kubernetes.Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed, nil
	})
}

// GetNetworkPolicy fetches a NetworkPolicy by namespace and name.
func GetNetworkPolicy(ctx context.Context, client kubernetes.Interface, namespace, name string) *networkingv1.NetworkPolicy {
	policy, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to get NetworkPolicy %s/%s", namespace, name))
	return policy
}

// RequirePodSelectorLabel asserts that the policy's podSelector contains the given key=value label.
func RequirePodSelectorLabel(policy *networkingv1.NetworkPolicy, key, value string) {
	actual, ok := policy.Spec.PodSelector.MatchLabels[key]
	o.Expect(ok && actual == value).To(o.BeTrue(), fmt.Sprintf("%s/%s: expected podSelector %s=%s, got %v", policy.Namespace, policy.Name, key, value, policy.Spec.PodSelector.MatchLabels))
}

// RequireOwnerReference asserts that the policy is owned by the given API object.
func RequireOwnerReference(policy *networkingv1.NetworkPolicy, apiVersion, kind, name string) {
	for _, ref := range policy.OwnerReferences {
		if ref.APIVersion == apiVersion && ref.Kind == kind && ref.Name == name {
			return
		}
	}
	g.Fail(fmt.Sprintf("%s/%s: expected ownerReference %s %s/%s, got %v", policy.Namespace, policy.Name, apiVersion, kind, name, policy.OwnerReferences))
}

// RequireIngressPort asserts that the policy has an ingress rule with the specified protocol and port.
func RequireIngressPort(policy *networkingv1.NetworkPolicy, protocol corev1.Protocol, port int32) {
	found := false
	for _, rule := range policy.Spec.Ingress {
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol != protocol {
				continue
			}
			if p.Port == nil || p.Port.IntValue() == int(port) {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("%s/%s: expected ingress port %s/%d", policy.Namespace, policy.Name, protocol, port))
}

// RequireUnrestrictedEgress asserts that the policy has at least one egress rule
// with no port and no destination restrictions.
func RequireUnrestrictedEgress(policy *networkingv1.NetworkPolicy) {
	o.Expect(len(policy.Spec.Egress) > 0).To(o.BeTrue(), fmt.Sprintf("%s/%s: expected at least one egress rule, got none", policy.Namespace, policy.Name))
	for _, rule := range policy.Spec.Egress {
		if len(rule.Ports) == 0 && len(rule.To) == 0 {
			return
		}
	}
	g.Fail(fmt.Sprintf("%s/%s: no unrestricted egress rule [{}] found among %d rules", policy.Namespace, policy.Name, len(policy.Spec.Egress)))
}

// RequireIngressFromNamespaceLabel asserts that the policy allows ingress from
// namespaces with the given label on the specified port.
func RequireIngressFromNamespaceLabel(policy *networkingv1.NetworkPolicy, port int32, key, value string) {
	found := false
	for _, rule := range policy.Spec.Ingress {
		// Check if port matches (TCP)
		portMatches := false
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol != corev1.ProtocolTCP {
				continue
			}
			if p.Port == nil || p.Port.IntValue() == int(port) {
				portMatches = true
				break
			}
		}
		if !portMatches {
			continue
		}
		// Check namespace labels
		for _, peer := range rule.From {
			if peer.NamespaceSelector == nil || peer.NamespaceSelector.MatchLabels == nil {
				continue
			}
			if actual, ok := peer.NamespaceSelector.MatchLabels[key]; ok && actual == value {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("%s/%s: expected ingress from namespaces with %s=%s on port %d", policy.Namespace, policy.Name, key, value, port))
}

// RequireIngressFromPolicyGroup asserts that the policy allows ingress from
// namespaces with the given policy-group label on the specified port.
func RequireIngressFromPolicyGroup(policy *networkingv1.NetworkPolicy, port int32, policyGroupLabelKey string) {
	found := false
	for _, rule := range policy.Spec.Ingress {
		// Check if port matches (TCP)
		portMatches := false
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol != corev1.ProtocolTCP {
				continue
			}
			if p.Port == nil || p.Port.IntValue() == int(port) {
				portMatches = true
				break
			}
		}
		if !portMatches {
			continue
		}
		// Check policy-group label existence
		for _, peer := range rule.From {
			if peer.NamespaceSelector == nil || peer.NamespaceSelector.MatchLabels == nil {
				continue
			}
			if _, ok := peer.NamespaceSelector.MatchLabels[policyGroupLabelKey]; ok {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	o.Expect(found).To(o.BeTrue(), fmt.Sprintf("%s/%s: expected ingress from policy-group %s on port %d", policy.Namespace, policy.Name, policyGroupLabelKey, port))
}

// RestoreNetworkPolicy deletes the given network policy and waits for the
// operator to recreate it with the expected spec.
func RestoreNetworkPolicy(ctx context.Context, client kubernetes.Interface, expected *networkingv1.NetworkPolicy, timeout time.Duration) {
	namespace := expected.Namespace
	name := expected.Name
	g.By(fmt.Sprintf("deleting NetworkPolicy %s/%s and waiting for restoration", namespace, name))
	err := client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to delete NetworkPolicy %s/%s", namespace, name))

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		current, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return apiequality.Semantic.DeepEqual(expected.Spec, current.Spec), nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("timed out waiting for NetworkPolicy %s/%s spec to be restored", namespace, name))
	g.By(fmt.Sprintf("NetworkPolicy %s/%s spec restored after delete", namespace, name))
}

// AssertUnmanagedNetworkPolicyPreserved waits through a reconcile window and
// fails if the operator deletes a custom NetworkPolicy it does not own.
func AssertUnmanagedNetworkPolicyPreserved(ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	g.By(fmt.Sprintf("waiting %s to confirm unmanaged NetworkPolicy %s/%s is not pruned", timeout, namespace, name))
	pollErr := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		_, getErr := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if k8serrors.IsNotFound(getErr) {
			return false, fmt.Errorf("operator deleted unmanaged NetworkPolicy %s/%s", namespace, name)
		}
		if getErr != nil {
			return false, nil
		}
		return false, nil
	})
	_, getErr := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if k8serrors.IsNotFound(getErr) {
		g.Fail(fmt.Sprintf("unmanaged NetworkPolicy %s/%s was deleted by the operator", namespace, name))
	}
	o.Expect(getErr).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to get unmanaged NetworkPolicy %s/%s", namespace, name))
	if pollErr != nil && !wait.Interrupted(pollErr) {
		g.Fail(fmt.Sprintf("unmanaged NetworkPolicy %s/%s was not preserved: %v", namespace, name, pollErr))
	}
	g.By(fmt.Sprintf("unmanaged NetworkPolicy %s/%s still exists", namespace, name))
}

// LogNetworkPolicyEvents searches for NetworkPolicy-related events (best-effort).
func LogNetworkPolicyEvents(ctx context.Context, client kubernetes.Interface, namespaces []string, policyName string) {
	found := false
	_ = wait.PollUntilContextTimeout(ctx, 5*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		for _, namespace := range namespaces {
			eventList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				g.By(fmt.Sprintf("unable to list events in %s: %v", namespace, err))
				continue
			}
			for _, event := range eventList.Items {
				isNPEvent := strings.HasPrefix(event.Reason, "NetworkPolicy") ||
					event.InvolvedObject.Kind == "NetworkPolicy" ||
					(policyName != "" && strings.Contains(event.Message, policyName))
				if isNPEvent {
					g.By(fmt.Sprintf("event in %s: type=%s reason=%s involvedObject=%s/%s message=%q",
						namespace, event.Type, event.Reason,
						event.InvolvedObject.Kind, event.InvolvedObject.Name,
						event.Message))
					found = true
				}
			}
		}
		if found {
			return true, nil
		}
		return false, nil
	})
	if !found {
		g.By(fmt.Sprintf("no NetworkPolicy events observed for %s (best-effort)", policyName))
	}
}

func waitForOperandNetworkPolicy(ctx context.Context, client kubernetes.Interface) {
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("timed out waiting for NetworkPolicy %s/%s", operatorclient.OperatorNamespace, operandNetworkPolicyName))
}

func waitForLeaderOperandPod(ctx context.Context, client kubernetes.Interface) *corev1.Pod {
	var leader *corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := getLeaderOperandPod(ctx, client)
		if err != nil {
			g.By(fmt.Sprintf("waiting for leader operand pod: %v", err))
			return false, nil
		}
		if pod.Status.PodIP == "" {
			g.By(fmt.Sprintf("leader operand pod %s has no IPs yet", pod.Name))
			return false, nil
		}
		leader = pod
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("timed out waiting for leader operand pod"))
	g.By(fmt.Sprintf("leader operand pod %s ips=%v node=%s", leader.Name, []string{leader.Status.PodIP}, leader.Spec.NodeName))
	return leader
}

func getLeaderOperandPod(ctx context.Context, client kubernetes.Interface) (*corev1.Pod, error) {
	lease, err := client.CoordinationV1().Leases(operatorclient.OperatorNamespace).Get(ctx, operandLeaseName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get lease %s/%s: %w", operatorclient.OperatorNamespace, operandLeaseName, err)
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity == "" {
		return nil, fmt.Errorf("lease %s/%s has no holderIdentity", operatorclient.OperatorNamespace, operandLeaseName)
	}

	holder := *lease.Spec.HolderIdentity
	podName := holder
	if i := strings.LastIndex(holder, "_"); i > 0 {
		podName = holder[:i]
	}

	pod, err := client.CoreV1().Pods(operatorclient.OperatorNamespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get leader pod %s from holderIdentity %s: %w", podName, holder, err)
	}
	return pod, nil
}

func operandClientLabels() map[string]string {
	return map[string]string{operandAppLabelKey: operatorclient.OperandName}
}

// testMutationRecovery tests that a NetworkPolicy patch is reconciled back to its original spec.
// It applies the patch, waits for the operator to restore the original spec, and asserts equality.
func testMutationRecovery(ctx context.Context, kubeClient kubernetes.Interface, patch []byte, timeout time.Duration) {
	original := GetNetworkPolicy(ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)
	_, err := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Patch(ctx, operandNetworkPolicyName, "application/merge-patch+json", patch, metav1.PatchOptions{})
	if err != nil {
		// Try JSON patch if merge patch fails
		_, err = kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Patch(ctx, operandNetworkPolicyName, "application/json-patch+json", patch, metav1.PatchOptions{})
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	// Wait for reconciliation using polling instead of sleep
	waitErr := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		current, err := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// Check if spec has been restored
		return apiequality.Semantic.DeepEqual(original.Spec, current.Spec), nil
	})
	o.Expect(waitErr).NotTo(o.HaveOccurred(), "NetworkPolicy spec should be restored after mutation within timeout")

	// Final verification
	current, _ := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
	o.Expect(original.Spec).To(o.Equal(current.Spec), "Policy spec should be restored after mutation")
}

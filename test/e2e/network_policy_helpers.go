package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
)

const (
	defaultAgnhostImage = "registry.k8s.io/e2e-test-images/agnhost:2.45"

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
	openshiftIngressNamespace     = "openshift-ingress"
	openshiftIngressDenyAllPolicy = "openshift-ingress-deny-all"
	openshiftDNSNamespace         = "openshift-dns"
	prometheusK8sServiceName      = "prometheus-k8s"
	operandMetricsServiceName     = "openshift-cli-manager-metrics"
	operandPluginServiceName      = "openshift-cli-manager"
	operandServiceMonitorName     = "openshift-cli-manager"
	operandMetricsPortName        = "cli-manager-metrics-port"
	connectivityTimeout           = 2 * time.Minute
	networkPolicyReconcileTimeout = 10 * time.Minute
)

// IsIPv6 returns true if the given IP string is an IPv6 address.
func IsIPv6(ip string) bool {
	return net.ParseIP(ip) != nil && strings.Contains(ip, ":")
}

// FormatIPPort formats an IP:port pair, using brackets for IPv6 addresses.
func FormatIPPort(ip string, port int32) string {
	if IsIPv6(ip) {
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

// PodIPs returns all IP addresses assigned to a pod (dual-stack aware).
func PodIPs(pod *corev1.Pod) []string {
	var ips []string
	for _, podIP := range pod.Status.PodIPs {
		if podIP.IP != "" {
			ips = append(ips, podIP.IP)
		}
	}
	if len(ips) == 0 && pod.Status.PodIP != "" {
		ips = append(ips, pod.Status.PodIP)
	}
	return ips
}

// ServiceClusterIPs returns all ClusterIPs for a service (dual-stack aware).
func ServiceClusterIPs(svc *corev1.Service) []string {
	if len(svc.Spec.ClusterIPs) > 0 {
		return svc.Spec.ClusterIPs
	}
	if svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != corev1.ClusterIPNone {
		return []string{svc.Spec.ClusterIP}
	}
	return nil
}

func boolptr(value bool) *bool {
	return &value
}

func int64ptr(value int64) *int64 {
	return &value
}

func namespaceExists(ctx context.Context, client kubernetes.Interface, namespace string) bool {
	_, err := client.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	return err == nil
}

func runConnectivityCheck(ctx context.Context, kubeClient kubernetes.Interface, namespace string, labels map[string]string, serverIP string, port int32, hostNetwork bool, nodeName string) (bool, error) {
	name := fmt.Sprintf("np-client-%s", rand.String(5))
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			HostNetwork:   hostNetwork,
			NodeName:      nodeName,
			Tolerations: []corev1.Toleration{
				{Operator: corev1.TolerationOpExists},
			},
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   boolptr(true),
				RunAsUser:      int64ptr(1001),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{
				{
					Name:  "connect",
					Image: defaultAgnhostImage,
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolptr(false),
						Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						RunAsNonRoot:             boolptr(true),
						RunAsUser:                int64ptr(1001),
					},
					Command: []string{"/agnhost"},
					Args: []string{
						"connect",
						"--protocol=tcp",
						"--timeout=5s",
						FormatIPPort(serverIP, port),
					},
				},
			},
		},
	}
	if hostNetwork {
		pod.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}

	_, err := kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}
	defer func() {
		_ = kubeClient.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}()

	if err := WaitForPodCompletion(ctx, kubeClient, namespace, name); err != nil {
		return false, err
	}
	completed, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if len(completed.Status.ContainerStatuses) == 0 {
		return false, fmt.Errorf("no container status recorded for pod %s", name)
	}
	terminated := completed.Status.ContainerStatuses[0].State.Terminated
	if terminated == nil {
		return false, fmt.Errorf("container in pod %s has not terminated", name)
	}
	return terminated.ExitCode == 0, nil
}

// ExpectConnectivity checks connectivity from a pod in the given namespace
// (with clientLabels) to each serverIP on the specified port.
func ExpectConnectivity(ctx context.Context, t testing.TB, kubeClient kubernetes.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool) {
	t.Helper()
	for _, ip := range serverIPs {
		family := "IPv4"
		if IsIPv6(ip) {
			family = "IPv6"
		}
		t.Logf("checking %s connectivity %s -> %s expected=%t", family, namespace, FormatIPPort(ip, port), shouldSucceed)
		if err := pollConnectivity(ctx, kubeClient, namespace, clientLabels, ip, port, shouldSucceed, false, "", connectivityTimeout); err != nil {
			t.Fatalf("connectivity check failed for %s %s -> %s (expected %t): %v", family, namespace, FormatIPPort(ip, port), shouldSucceed, err)
		}
	}
}

// ExpectHostNetworkConnectivity checks connectivity from a host-network pod on
// the given node. Kubelet / host-network traffic bypasses NetworkPolicy.
func ExpectHostNetworkConnectivity(ctx context.Context, t testing.TB, kubeClient kubernetes.Interface, namespace, nodeName string, serverIPs []string, port int32, shouldSucceed bool) {
	t.Helper()
	for _, ip := range serverIPs {
		family := "IPv4"
		if IsIPv6(ip) {
			family = "IPv6"
		}
		t.Logf("checking %s host-network connectivity node=%s -> %s expected=%t", family, nodeName, FormatIPPort(ip, port), shouldSucceed)
		if err := pollConnectivity(ctx, kubeClient, namespace, nil, ip, port, shouldSucceed, true, nodeName, connectivityTimeout); err != nil {
			t.Fatalf("host-network connectivity check failed for %s node=%s -> %s (expected %t): %v", family, nodeName, FormatIPPort(ip, port), shouldSucceed, err)
		}
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

// WaitForPodCompletion waits up to 2 minutes for a pod to reach Succeeded or Failed.
func WaitForPodCompletion(ctx context.Context, kubeClient kubernetes.Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed, nil
	})
}

// HasPort returns true if the given list of NetworkPolicy ports includes a port
// matching the specified protocol and port number.
func HasPort(ports []networkingv1.NetworkPolicyPort, protocol corev1.Protocol, port int32) bool {
	for _, p := range ports {
		if p.Protocol != nil && *p.Protocol != protocol {
			continue
		}
		if p.Port == nil || p.Port.IntValue() == int(port) {
			return true
		}
	}
	return false
}

// HasPortInIngress returns true if any ingress rule contains the specified protocol/port.
func HasPortInIngress(rules []networkingv1.NetworkPolicyIngressRule, protocol corev1.Protocol, port int32) bool {
	for _, rule := range rules {
		if HasPort(rule.Ports, protocol, port) {
			return true
		}
	}
	return false
}

// HasIngressFromNamespace returns true if any ingress rule allows traffic from
// the specified namespace on the given port (TCP) via kubernetes.io/metadata.name.
func HasIngressFromNamespace(rules []networkingv1.NetworkPolicyIngressRule, port int32, namespace string) bool {
	return HasIngressFromNamespaceLabel(rules, port, "kubernetes.io/metadata.name", namespace)
}

// HasIngressFromNamespaceLabel returns true if any ingress rule allows traffic
// from namespaces with the given label on the specified port.
func HasIngressFromNamespaceLabel(rules []networkingv1.NetworkPolicyIngressRule, port int32, key, value string) bool {
	for _, rule := range rules {
		if !HasPort(rule.Ports, corev1.ProtocolTCP, port) {
			continue
		}
		for _, peer := range rule.From {
			if peer.NamespaceSelector == nil || peer.NamespaceSelector.MatchLabels == nil {
				continue
			}
			if actual, ok := peer.NamespaceSelector.MatchLabels[key]; ok && actual == value {
				return true
			}
		}
	}
	return false
}

// HasIngressFromPolicyGroup returns true if any ingress rule allows traffic
// from namespaces with the given policy-group label key on the specified port.
func HasIngressFromPolicyGroup(rules []networkingv1.NetworkPolicyIngressRule, port int32, policyGroupLabelKey string) bool {
	for _, rule := range rules {
		if !HasPort(rule.Ports, corev1.ProtocolTCP, port) {
			continue
		}
		for _, peer := range rule.From {
			if peer.NamespaceSelector == nil || peer.NamespaceSelector.MatchLabels == nil {
				continue
			}
			if _, ok := peer.NamespaceSelector.MatchLabels[policyGroupLabelKey]; ok {
				return true
			}
		}
	}
	return false
}

// GetNetworkPolicy fetches a NetworkPolicy by namespace and name.
func GetNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string) *networkingv1.NetworkPolicy {
	t.Helper()
	policy, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get NetworkPolicy %s/%s: %v", namespace, name, err)
	}
	return policy
}

// RequirePodSelectorLabel asserts that the policy's podSelector contains the given key=value label.
func RequirePodSelectorLabel(t testing.TB, policy *networkingv1.NetworkPolicy, key, value string) {
	t.Helper()
	actual, ok := policy.Spec.PodSelector.MatchLabels[key]
	if !ok || actual != value {
		t.Fatalf("%s/%s: expected podSelector %s=%s, got %v", policy.Namespace, policy.Name, key, value, policy.Spec.PodSelector.MatchLabels)
	}
}

// RequireOwnerReference asserts that the policy is owned by the given API object.
func RequireOwnerReference(t testing.TB, policy *networkingv1.NetworkPolicy, apiVersion, kind, name string) {
	t.Helper()
	for _, ref := range policy.OwnerReferences {
		if ref.APIVersion == apiVersion && ref.Kind == kind && ref.Name == name {
			return
		}
	}
	t.Fatalf("%s/%s: expected ownerReference %s %s/%s, got %v", policy.Namespace, policy.Name, apiVersion, kind, name, policy.OwnerReferences)
}

// RequireIngressPort asserts that the policy has an ingress rule with the specified protocol and port.
func RequireIngressPort(t testing.TB, policy *networkingv1.NetworkPolicy, protocol corev1.Protocol, port int32) {
	t.Helper()
	if !HasPortInIngress(policy.Spec.Ingress, protocol, port) {
		t.Fatalf("%s/%s: expected ingress port %s/%d", policy.Namespace, policy.Name, protocol, port)
	}
}

// RequireUnrestrictedEgress asserts that the policy has at least one egress rule
// with no port and no destination restrictions.
func RequireUnrestrictedEgress(t testing.TB, policy *networkingv1.NetworkPolicy) {
	t.Helper()
	if len(policy.Spec.Egress) == 0 {
		t.Fatalf("%s/%s: expected at least one egress rule, got none", policy.Namespace, policy.Name)
	}
	for _, rule := range policy.Spec.Egress {
		if len(rule.Ports) == 0 && len(rule.To) == 0 {
			return
		}
	}
	t.Fatalf("%s/%s: no unrestricted egress rule [{}] found among %d rules", policy.Namespace, policy.Name, len(policy.Spec.Egress))
}

// RequireIngressFromNamespace asserts that the policy allows ingress from the specified namespace on the given port.
func RequireIngressFromNamespace(t testing.TB, policy *networkingv1.NetworkPolicy, port int32, namespace string) {
	t.Helper()
	if !HasIngressFromNamespace(policy.Spec.Ingress, port, namespace) {
		t.Fatalf("%s/%s: expected ingress from namespace %s on port %d", policy.Namespace, policy.Name, namespace, port)
	}
}

// RequireIngressFromNamespaceLabel asserts that the policy allows ingress from
// namespaces with the given label on the specified port.
func RequireIngressFromNamespaceLabel(t testing.TB, policy *networkingv1.NetworkPolicy, port int32, key, value string) {
	t.Helper()
	if !HasIngressFromNamespaceLabel(policy.Spec.Ingress, port, key, value) {
		t.Fatalf("%s/%s: expected ingress from namespaces with %s=%s on port %d", policy.Namespace, policy.Name, key, value, port)
	}
}

// RequireIngressFromPolicyGroup asserts that the policy allows ingress from
// namespaces with the given policy-group label on the specified port.
func RequireIngressFromPolicyGroup(t testing.TB, policy *networkingv1.NetworkPolicy, port int32, policyGroupLabelKey string) {
	t.Helper()
	if !HasIngressFromPolicyGroup(policy.Spec.Ingress, port, policyGroupLabelKey) {
		t.Fatalf("%s/%s: expected ingress from policy-group %s on port %d", policy.Namespace, policy.Name, policyGroupLabelKey, port)
	}
}

// RestoreNetworkPolicy deletes the given network policy and waits for the
// operator to recreate it with the expected spec.
func RestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, expected *networkingv1.NetworkPolicy, timeout time.Duration) {
	t.Helper()
	namespace := expected.Namespace
	name := expected.Name
	t.Logf("deleting NetworkPolicy %s/%s and waiting for restoration", namespace, name)
	if err := client.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("failed to delete NetworkPolicy %s/%s: %v", namespace, name, err)
	}
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		current, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return apiequality.Semantic.DeepEqual(expected.Spec, current.Spec), nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for NetworkPolicy %s/%s spec to be restored", namespace, name)
	}
	t.Logf("NetworkPolicy %s/%s spec restored after delete", namespace, name)
}

// MutateAndRestoreNetworkPolicy patches the policy's podSelector with a
// spurious label, then waits for the operator to reconcile it back.
func MutateAndRestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	t.Helper()
	patch := []byte(`{"spec":{"podSelector":{"matchLabels":{"np-reconcile":"mutated"}}}}`)
	mutateAndRestoreNetworkPolicy(t, ctx, client, namespace, name, types.MergePatchType, patch, timeout, "podSelector override")
}

// MutatePortAndRestoreNetworkPolicy changes the metrics ingress port and waits
// for the operator to restore the original spec.
func MutatePortAndRestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	t.Helper()
	patch := []byte(`[{"op":"replace","path":"/spec/ingress/0/ports/0/port","value":9999}]`)
	mutateAndRestoreNetworkPolicy(t, ctx, client, namespace, name, types.JSONPatchType, patch, timeout, "ingress port override")
}

// MutatePolicyTypesAndRestoreNetworkPolicy drops Egress from policyTypes and
// waits for the operator to restore Ingress+Egress.
func MutatePolicyTypesAndRestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	t.Helper()
	patch := []byte(`[{"op":"replace","path":"/spec/policyTypes","value":["Ingress"]}]`)
	mutateAndRestoreNetworkPolicy(t, ctx, client, namespace, name, types.JSONPatchType, patch, timeout, "policyTypes override")
}

// MutateNamespaceSelectorAndRestoreNetworkPolicy flips the monitoring
// namespaceSelector and waits for the operator to restore it.
func MutateNamespaceSelectorAndRestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	t.Helper()
	patch := []byte(`[{"op":"replace","path":"/spec/ingress/0/from/0/namespaceSelector/matchLabels/openshift.io~1cluster-monitoring","value":"false"}]`)
	mutateAndRestoreNetworkPolicy(t, ctx, client, namespace, name, types.JSONPatchType, patch, timeout, "namespaceSelector override")
}

// MutateEmptyIngressAndRestoreNetworkPolicy clears all ingress rules and waits
// for the operator to restore them.
func MutateEmptyIngressAndRestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	t.Helper()
	patch := []byte(`[{"op":"replace","path":"/spec/ingress","value":[]}]`)
	mutateAndRestoreNetworkPolicy(t, ctx, client, namespace, name, types.JSONPatchType, patch, timeout, "empty ingress override")
}

// AssertUnmanagedNetworkPolicyPreserved waits through a reconcile window and
// fails if the operator deletes a custom NetworkPolicy it does not own.
func AssertUnmanagedNetworkPolicyPreserved(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, timeout time.Duration) {
	t.Helper()
	t.Logf("waiting %s to confirm unmanaged NetworkPolicy %s/%s is not pruned", timeout, namespace, name)
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
		t.Fatalf("unmanaged NetworkPolicy %s/%s was deleted by the operator", namespace, name)
	}
	if getErr != nil {
		t.Fatalf("failed to get unmanaged NetworkPolicy %s/%s: %v", namespace, name, getErr)
	}
	if pollErr != nil && !wait.Interrupted(pollErr) {
		t.Fatalf("unmanaged NetworkPolicy %s/%s was not preserved: %v", namespace, name, pollErr)
	}
	t.Logf("unmanaged NetworkPolicy %s/%s still exists", namespace, name)
}

func mutateAndRestoreNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface, namespace, name string, patchType types.PatchType, patch []byte, timeout time.Duration, description string) {
	t.Helper()
	original := GetNetworkPolicy(t, ctx, client, namespace, name)
	t.Logf("mutating NetworkPolicy %s/%s (%s) and waiting for reconciliation", namespace, name, description)
	_, err := client.NetworkingV1().NetworkPolicies(namespace).Patch(ctx, name, patchType, patch, metav1.PatchOptions{})
	if err != nil {
		t.Fatalf("failed to patch NetworkPolicy %s/%s: %v", namespace, name, err)
	}

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		current, getErr := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
		if getErr != nil {
			return false, nil
		}
		return apiequality.Semantic.DeepEqual(original.Spec, current.Spec), nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for NetworkPolicy %s/%s spec to be restored after %s", namespace, name, description)
	}
	t.Logf("NetworkPolicy %s/%s spec restored after %s", namespace, name, description)
}

// LogNetworkPolicyEvents searches for NetworkPolicy-related events (best-effort).
func LogNetworkPolicyEvents(t testing.TB, ctx context.Context, client kubernetes.Interface, namespaces []string, policyName string) {
	t.Helper()
	found := false
	_ = wait.PollUntilContextTimeout(ctx, 5*time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		for _, namespace := range namespaces {
			eventList, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				t.Logf("unable to list events in %s: %v", namespace, err)
				continue
			}
			for _, event := range eventList.Items {
				isNPEvent := strings.HasPrefix(event.Reason, "NetworkPolicy") ||
					event.InvolvedObject.Kind == "NetworkPolicy" ||
					(policyName != "" && strings.Contains(event.Message, policyName))
				if isNPEvent {
					t.Logf("event in %s: type=%s reason=%s involvedObject=%s/%s message=%q",
						namespace, event.Type, event.Reason,
						event.InvolvedObject.Kind, event.InvolvedObject.Name,
						event.Message)
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
		t.Logf("no NetworkPolicy events observed for %s (best-effort)", policyName)
	}
}

func waitForOperandNetworkPolicy(t testing.TB, ctx context.Context, client kubernetes.Interface) {
	t.Helper()
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
	if err != nil {
		t.Fatalf("timed out waiting for NetworkPolicy %s/%s", operatorclient.OperatorNamespace, operandNetworkPolicyName)
	}
}

func waitForLeaderOperandPod(t testing.TB, ctx context.Context, client kubernetes.Interface) *corev1.Pod {
	t.Helper()
	var leader *corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := getLeaderOperandPod(ctx, client)
		if err != nil {
			t.Logf("waiting for leader operand pod: %v", err)
			return false, nil
		}
		if len(PodIPs(pod)) == 0 {
			t.Logf("leader operand pod %s has no IPs yet", pod.Name)
			return false, nil
		}
		leader = pod
		return true, nil
	})
	if err != nil {
		t.Fatalf("timed out waiting for leader operand pod: %v", err)
	}
	t.Logf("leader operand pod %s ips=%v node=%s", leader.Name, PodIPs(leader), leader.Spec.NodeName)
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

func ingressNamespaceHasDenyAll(ctx context.Context, client kubernetes.Interface) bool {
	_, err := client.NetworkingV1().NetworkPolicies(openshiftIngressNamespace).Get(ctx, openshiftIngressDenyAllPolicy, metav1.GetOptions{})
	return err == nil
}

// createTempIngressNamespace creates a temporary namespace with the
// policy-group.network.openshift.io/ingress label so connectivity tests
// don't depend on openshift-ingress, which may have deny-all policies
// blocking non-router pods (OCP 5.x+).
func createTempIngressNamespace(t testing.TB, ctx context.Context, client kubernetes.Interface) (string, func()) {
	t.Helper()
	name := fmt.Sprintf("np-ingress-test-%s", rand.String(5))
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				ingressPolicyGroupKey:                        "",
				"pod-security.kubernetes.io/enforce":         "restricted",
				"pod-security.kubernetes.io/enforce-version": "latest",
			},
		},
	}
	_, err := client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create temp ingress namespace %s: %v", name, err)
	}
	t.Logf("created temp namespace %s with label %s", name, ingressPolicyGroupKey)
	cleanup := func() {
		if delErr := client.CoreV1().Namespaces().Delete(context.Background(), name, metav1.DeleteOptions{}); delErr != nil {
			t.Logf("failed to delete temp namespace %s: %v", name, delErr)
		}
	}
	return name, cleanup
}

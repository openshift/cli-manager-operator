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
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
)

//go:embed testdata/connectivity-test-pod.yaml
var testData embed.FS

const (
	operandNetworkPolicyName = "cli-manager-operand"
	operandAppLabelKey       = "app"
	operandLeaseName         = "cli-manager-lock"
	operatorDeploymentName   = "openshift-cli-manager-operator"

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

var _ = g.Describe("[Operator][Serial] CLI Manager NetworkPolicy", g.Ordered, func() {
	var (
		ctx        context.Context
		cancelFnc  context.CancelFunc
		kubeClient *k8sclient.Clientset
	)

	g.BeforeAll(func() {
		g.By("Setting up the CLI Manager operator")
		var err error
		ctx, cancelFnc, kubeClient, err = setupOperator(g.GinkgoTB())
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for the operand NetworkPolicy and leader pod")
		waitForOperandNetworkPolicy(ctx, kubeClient)
		waitForLeaderOperandPod(ctx, kubeClient)
	})

	g.AfterAll(func() {
		teardownOperator()
		if cancelFnc != nil {
			cancelFnc()
		}
	})

	g.It("should define operand NetworkPolicy with correct structure and selectors", func() {
		expectNetworkPolicyValid(ctx, kubeClient, "NetworkPolicy should exist with correct structure and selectors")
	})

	g.It("should enforce ingress policy for metrics, plugin, and health ports", func() {
		leader := waitForLeaderOperandPod(ctx, kubeClient)
		leaderIPs := []string{leader.Status.PodIP}
		testLabels := map[string]string{"test": "cli-manager-netpol"}

		ingressTests := []struct {
			description string
			namespace   string
			port        int32
			shouldAllow bool
		}{
			{
				description: "metrics port allowed from openshift-monitoring",
				namespace:   openshiftMonitoringNamespace,
				port:        metricsPort,
				shouldAllow: true,
			},
			{
				description: "metrics port allowed from openshift-user-workload-monitoring",
				namespace:   openshiftUWMNamespace,
				port:        metricsPort,
				shouldAllow: true,
			},
			{
				description: "metrics port allowed from operator namespace (cluster-monitoring label)",
				namespace:   operatorclient.OperatorNamespace,
				port:        metricsPort,
				shouldAllow: true,
			},
			{
				description: "metrics port blocked from default namespace",
				namespace:   "default",
				port:        metricsPort,
				shouldAllow: false,
			},
			{
				description: "wrong port blocked from monitoring namespace",
				namespace:   openshiftMonitoringNamespace,
				port:        unusedPort,
				shouldAllow: false,
			},
			{
				description: "plugin port blocked from default namespace",
				namespace:   "default",
				port:        pluginPort,
				shouldAllow: false,
			},
			{
				description: "health port blocked from monitoring namespace",
				namespace:   openshiftMonitoringNamespace,
				port:        healthPort,
				shouldAllow: false,
			},
		}

		for _, tc := range ingressTests {
			g.By(fmt.Sprintf("%s", tc.description))
			expectConnectivity(ctx, kubeClient, tc.namespace, testLabels, leaderIPs, tc.port, tc.shouldAllow)
		}
	})

	g.It("should allow kubelet/host-network to bypass NetworkPolicy", func() {
		leader := waitForLeaderOperandPod(ctx, kubeClient)
		leaderIPs := []string{leader.Status.PodIP}

		g.By("Kubelet/host-network should bypass metrics port policy")
		expectHostNetworkConnectivity(ctx, kubeClient, openshiftMonitoringNamespace, leader.Spec.NodeName, leaderIPs, metricsPort, true)
	})

	g.It("should allow unrestricted egress for DNS, API server, and prometheus", func() {
		clientLabels := operandClientLabels()

		dnsSvc, err := kubeClient.CoreV1().Services(openshiftDNSNamespace).Get(ctx, "dns-default", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		dnsIPs := serviceClusterIPs(dnsSvc)

		g.By("DNS: Allowed to openshiftDNSNamespace/dns-default:53")
		expectConnectivity(ctx, kubeClient, operatorclient.OperatorNamespace, clientLabels, dnsIPs, 53, true)

		kubeSvc, err := kubeClient.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		kubeIPs := serviceClusterIPs(kubeSvc)

		g.By("Kubernetes API: Allowed to default/kubernetes:443")
		expectConnectivity(ctx, kubeClient, operatorclient.OperatorNamespace, clientLabels, kubeIPs, 443, true)

		promSvc, err := kubeClient.CoreV1().Services(openshiftMonitoringNamespace).Get(ctx, prometheusK8sServiceName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		promPort := int32(9091)
		for _, p := range promSvc.Spec.Ports {
			if p.Name == "web" || p.Port == 9091 {
				promPort = p.Port
				break
			}
		}
		promIPs := serviceClusterIPs(promSvc)

		g.By(fmt.Sprintf("Prometheus: Allowed to prometheus-k8s:%d", promPort))
		expectConnectivity(ctx, kubeClient, operatorclient.OperatorNamespace, clientLabels, promIPs, promPort, true)
	})

	g.It("should reconcile policy mutations and restore deleted policies", func() {
		expected := getNetworkPolicy(ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)

		g.By("Testing reconciliation of port mutation")
		patch := []byte(`[{"op":"replace","path":"/spec/ingress/0/ports/0/port","value":9999}]`)
		testMutationRecovery(ctx, kubeClient, patch, networkPolicyReconcileTimeout)

		expectNetworkPolicyValid(ctx, kubeClient, "operator should revert port mutation")

		g.By("Testing deletion and recreation of policy")
		restoreNetworkPolicy(ctx, kubeClient, expected, networkPolicyReconcileTimeout)

		logNetworkPolicyEvents(ctx, kubeClient, []string{operatorclient.OperatorNamespace}, operandNetworkPolicyName)
	})

	g.It("should recover NetworkPolicy after config drift on operator restart", func() {
		netpolClient := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace)

		g.By("Scaling down operator to 0 replicas")
		scaleDeployment(ctx, kubeClient, operatorclient.OperatorNamespace, operatorDeploymentName, 0)
		verifyPodCount(ctx, kubeClient, operatorclient.OperatorNamespace, "name="+operatorDeploymentName, 0)

		g.By("Wiping all ingress rules from NetworkPolicy")
		patch := []byte(`[{"op": "replace", "path": "/spec/ingress", "value": []}]`)
		_, err := netpolClient.Patch(ctx, operandNetworkPolicyName, "application/json-patch+json", patch, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to wipe ingress rules")

		np, err := netpolClient.Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get NetworkPolicy after wipe")
		o.Expect(np.Spec.Ingress).To(o.BeEmpty(), "expected 0 ingress rules after wipe")

		g.By("Scaling operator back up to 1 replica")
		scaleDeployment(ctx, kubeClient, operatorclient.OperatorNamespace, operatorDeploymentName, 1)

		o.Eventually(func() error {
			deploy, err := kubeClient.AppsV1().Deployments(operatorclient.OperatorNamespace).Get(ctx, operatorDeploymentName, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if deploy.Status.ReadyReplicas < 1 {
				return fmt.Errorf("operator not ready yet: %d ready replicas", deploy.Status.ReadyReplicas)
			}
			return nil
		}, 2*time.Minute, 2*time.Second).Should(o.Succeed(), "operator should become ready")

		expectNetworkPolicyValid(ctx, kubeClient, "operator should recover NetworkPolicy after restart")
	})
})

// expectNetworkPolicyValid waits for NetworkPolicy to exist and be valid
func expectNetworkPolicyValid(ctx context.Context, kubeClient k8sclient.Interface, msg string) {
	o.Eventually(func() error {
		netpol, err := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return validateNetworkPolicySpec(netpol)
	}, 1*time.Minute, 2*time.Second).Should(o.Succeed(), msg)
}

// validateNetworkPolicySpec validates the complete NetworkPolicy specification
func validateNetworkPolicySpec(netpol *networkingv1.NetworkPolicy) error {
	if netpol.Namespace != operatorclient.OperatorNamespace {
		return fmt.Errorf("policy namespace should be %s, got %s", operatorclient.OperatorNamespace, netpol.Namespace)
	}

	sel := netpol.Spec.PodSelector
	if sel.MatchLabels[operandAppLabelKey] != operatorclient.OperandName {
		return fmt.Errorf("invalid podSelector label: expected %s=%s, got %v", operandAppLabelKey, operatorclient.OperandName, sel.MatchLabels)
	}

	metricsPortFound := false
	pluginPortFound := false

	for _, rule := range netpol.Spec.Ingress {
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol != corev1.ProtocolTCP {
				continue
			}
			if p.Port != nil {
				if p.Port.IntValue() == int(metricsPort) {
					metricsPortFound = true
				}
				if p.Port.IntValue() == int(pluginPort) {
					pluginPortFound = true
				}
			}
		}
	}

	if !metricsPortFound {
		return fmt.Errorf("metrics port %d not found in ingress rules", metricsPort)
	}
	if !pluginPortFound {
		return fmt.Errorf("plugin port %d not found in ingress rules", pluginPort)
	}

	metricsNamespaceLabelFound := false
	metricsClusterMonitoringFound := false
	pluginPolicyGroupFound := false

	for _, rule := range netpol.Spec.Ingress {
		for _, peer := range rule.From {
			if peer.NamespaceSelector != nil && peer.NamespaceSelector.MatchLabels != nil {
				if peer.NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"] == openshiftMonitoringNamespace {
					metricsNamespaceLabelFound = true
				}
				if peer.NamespaceSelector.MatchLabels[clusterMonitoringLabel] == "true" {
					metricsClusterMonitoringFound = true
				}
				if _, ok := peer.NamespaceSelector.MatchLabels[ingressPolicyGroupKey]; ok {
					pluginPolicyGroupFound = true
				}
			}
		}
	}

	if !metricsNamespaceLabelFound {
		return fmt.Errorf("expected ingress from namespace %s not found", openshiftMonitoringNamespace)
	}
	if !metricsClusterMonitoringFound {
		return fmt.Errorf("expected ingress from cluster-monitoring label not found")
	}
	if !pluginPolicyGroupFound {
		return fmt.Errorf("expected ingress from policy-group %s not found", ingressPolicyGroupKey)
	}

	egressUnrestrictedFound := false
	for _, rule := range netpol.Spec.Egress {
		if len(rule.Ports) == 0 && len(rule.To) == 0 {
			egressUnrestrictedFound = true
			break
		}
	}
	if !egressUnrestrictedFound {
		return fmt.Errorf("unrestricted egress rule not found")
	}

	if !contains(netpol.Spec.PolicyTypes, networkingv1.PolicyTypeIngress) {
		return fmt.Errorf("PolicyTypeIngress not found in policyTypes")
	}
	if !contains(netpol.Spec.PolicyTypes, networkingv1.PolicyTypeEgress) {
		return fmt.Errorf("PolicyTypeEgress not found in policyTypes")
	}

	ownerFound := false
	for _, ref := range netpol.OwnerReferences {
		if ref.APIVersion == "operator.openshift.io/v1" && ref.Kind == "CliManager" && ref.Name == operatorclient.OperatorConfigName {
			ownerFound = true
			break
		}
	}
	if !ownerFound {
		return fmt.Errorf("expected owner reference not found")
	}

	return nil
}

func contains(types []networkingv1.PolicyType, pType networkingv1.PolicyType) bool {
	for _, t := range types {
		if t == pType {
			return true
		}
	}
	return false
}

func serviceClusterIPs(svc *corev1.Service) []string {
	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == corev1.ClusterIPNone {
		return nil
	}
	return []string{svc.Spec.ClusterIP}
}

func runConnectivityCheck(ctx context.Context, kubeClient k8sclient.Interface, namespace string, labels map[string]string, serverIP string, port int32, hostNetwork bool, nodeName string) (bool, error) {
	podYAML, err := testData.ReadFile("testdata/connectivity-test-pod.yaml")
	if err != nil {
		return false, fmt.Errorf("failed to load embedded pod manifest: %w", err)
	}

	podYAML = []byte(strings.ReplaceAll(string(podYAML), "{{NAMESPACE}}", namespace))
	podYAML = []byte(strings.ReplaceAll(string(podYAML), "{{TARGET}}", fmt.Sprintf("%s:%d", serverIP, port)))

	pod := &corev1.Pod{}
	if err := yaml.Unmarshal(podYAML, pod); err != nil {
		return false, fmt.Errorf("failed to unmarshal pod manifest: %w", err)
	}

	pod.Labels = labels
	pod.Spec.NodeName = nodeName
	pod.Spec.HostNetwork = hostNetwork

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

	if err := waitForPodCompletion(ctx, kubeClient, namespace, podName); err != nil {
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

func expectConnectivity(ctx context.Context, kubeClient k8sclient.Interface, namespace string, clientLabels map[string]string, serverIPs []string, port int32, shouldSucceed bool) {
	for _, ip := range serverIPs {
		g.By(fmt.Sprintf("checking IPv4 connectivity %s -> %s:%d expected=%t", namespace, ip, port, shouldSucceed))
		err := pollConnectivity(ctx, kubeClient, namespace, clientLabels, ip, port, shouldSucceed, false, "", connectivityTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("connectivity check failed for %s -> %s:%d (expected %t)", namespace, ip, port, shouldSucceed))
	}
}

func expectHostNetworkConnectivity(ctx context.Context, kubeClient k8sclient.Interface, namespace, nodeName string, serverIPs []string, port int32, shouldSucceed bool) {
	for _, ip := range serverIPs {
		g.By(fmt.Sprintf("checking IPv4 host-network connectivity node=%s -> %s:%d expected=%t", nodeName, ip, port, shouldSucceed))
		err := pollConnectivity(ctx, kubeClient, namespace, nil, ip, port, shouldSucceed, true, nodeName, connectivityTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("host-network connectivity check failed for node=%s -> %s:%d (expected %t)", nodeName, ip, port, shouldSucceed))
	}
}

func pollConnectivity(ctx context.Context, kubeClient k8sclient.Interface, namespace string, clientLabels map[string]string, serverIP string, port int32, shouldSucceed, hostNetwork bool, nodeName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, timeout, true, func(_ context.Context) (bool, error) {
		succeeded, err := runConnectivityCheck(ctx, kubeClient, namespace, clientLabels, serverIP, port, hostNetwork, nodeName)
		if err != nil {
			return false, nil
		}
		return succeeded == shouldSucceed, nil
	})
}

func waitForPodCompletion(ctx context.Context, kubeClient k8sclient.Interface, namespace, name string) error {
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed, nil
	})
}

func getNetworkPolicy(ctx context.Context, client k8sclient.Interface, namespace, name string) *networkingv1.NetworkPolicy {
	policy, err := client.NetworkingV1().NetworkPolicies(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to get NetworkPolicy %s/%s", namespace, name))
	return policy
}

func restoreNetworkPolicy(ctx context.Context, client k8sclient.Interface, expected *networkingv1.NetworkPolicy, timeout time.Duration) {
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

func logNetworkPolicyEvents(ctx context.Context, client k8sclient.Interface, namespaces []string, policyName string) {
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

func waitForOperandNetworkPolicy(ctx context.Context, client k8sclient.Interface) {
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

func waitForLeaderOperandPod(ctx context.Context, client k8sclient.Interface) *corev1.Pod {
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

func getLeaderOperandPod(ctx context.Context, client k8sclient.Interface) (*corev1.Pod, error) {
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

func testMutationRecovery(ctx context.Context, kubeClient k8sclient.Interface, patch []byte, timeout time.Duration) {
	original := getNetworkPolicy(ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)
	_, err := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Patch(ctx, operandNetworkPolicyName, "application/merge-patch+json", patch, metav1.PatchOptions{})
	if err != nil {
		_, err = kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Patch(ctx, operandNetworkPolicyName, "application/json-patch+json", patch, metav1.PatchOptions{})
	}
	o.Expect(err).NotTo(o.HaveOccurred())

	waitErr := wait.PollUntilContextTimeout(ctx, 500*time.Millisecond, 30*time.Second, true, func(ctx context.Context) (bool, error) {
		current, err := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return apiequality.Semantic.DeepEqual(original.Spec, current.Spec), nil
	})
	o.Expect(waitErr).NotTo(o.HaveOccurred(), "NetworkPolicy spec should be restored after mutation within timeout")

	current, _ := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Get(ctx, operandNetworkPolicyName, metav1.GetOptions{})
	o.Expect(original.Spec).To(o.Equal(current.Spec), "Policy spec should be restored after mutation")
}

func scaleDeployment(ctx context.Context, kubeClient k8sclient.Interface, namespace, deploymentName string, replicas int32) {
	deploy, err := kubeClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	deploy.Spec.Replicas = &replicas
	_, err = kubeClient.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
}

func verifyPodCount(ctx context.Context, kubeClient k8sclient.Interface, namespace, labelSelector string, expectedCount int) {
	o.Eventually(func() int {
		pods, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return -1
		}
		return len(pods.Items)
	}, 1*time.Minute, 2*time.Second).Should(o.Equal(expectedCount), fmt.Sprintf("expected %d pods with selector %s", expectedCount, labelSelector))
}

package e2e

// NetworkPolicy e2e coverage for the operand policy
// allow-all-egress-and-metrics-ingress-operand. Specs are Ginkgo/OTE only,
// adapted to CLI Manager namespaces, labels, and ports:
//
//	ns:    openshift-cli-manager-operator (has openshift.io/cluster-monitoring=true)
//	pod:   app=openshift-cli-manager (leader via lease cli-manager-lock)
//	ports: metrics 60000, plugin 9449 (ingress-restricted, not JobSet unrestricted 9443), health 8443
//
//	1/2   policy created with expected selectors/ports/egress/policyTypes/ownerRef
//	3     custom unmanaged NetworkPolicy is not pruned
//	3.1-3.5 metrics ingress on 60000 (monitoring allowed, default/wrong-port denied;
//	        same-ns ALLOWED because this operator ns has cluster-monitoring)
//	3.6-3.7, 3.10 plugin ingress on 9449 (ingress ns allowed, others denied)
//	3.8-3.9 host-network allowed; health port 8443 denied from monitoring
//	4.1-4.4 unrestricted egress (DNS, API, internet, prometheus)
//	5     ServiceMonitor and metrics/plugin services
//	7.1-7.6 reconciliation after port/selector/policyTypes/ns-selector/empty-ingress/delete
//	8.1-8.2 unlabeled pods are not selected (no default-deny)

import (
	"context"
	"testing"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
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
		waitForOperandNetworkPolicy(g.GinkgoTB(), ctx, kubeClient)
		waitForLeaderOperandPod(g.GinkgoTB(), ctx, kubeClient)
	})

	g.AfterAll(func() {
		teardownOperator()
		if cancelFnc != nil {
			cancelFnc()
		}
	})

	g.It("should ensure operand NetworkPolicy is defined [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandNetworkPolicyDefined(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should enforce operand metrics ingress on port 60000 [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandMetricsIngress(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should enforce operand plugin download ingress on port 9449 [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandPluginIngress(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should block operand health ingress and allow host-network [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandHealthAndHostNetworkIngress(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should allow operand egress connectivity [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandEgress(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should configure ServiceMonitor for operand metrics [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandServiceMonitor(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should not apply operand NetworkPolicy to unlabeled pods [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testUnlabeledPodEgress(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should preserve unmanaged custom NetworkPolicies [NetworkPolicy] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testCustomNetworkPolicyPreserved(g.GinkgoTB(), ctx, kubeClient)
	})

	g.It("should restore operand NetworkPolicy after delete or mutation [NetworkPolicy][Timeout:30m][Disruptive] [Suite:openshift/cli-manager-operator/operator/serial]", func() {
		testOperandNetworkPolicyReconciliation(g.GinkgoTB(), ctx, kubeClient)
	})
})

func testOperandNetworkPolicyDefined(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	t.Logf("=== Validating %s ===", operandNetworkPolicyName)

	policy := GetNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)
	t.Logf(" - Policy found: %s/%s", policy.Namespace, policy.Name)

	RequirePodSelectorLabel(t, policy, operandAppLabelKey, operatorclient.OperandName)
	t.Logf(" - PodSelector: %s=%s", operandAppLabelKey, operatorclient.OperandName)

	RequireIngressPort(t, policy, corev1.ProtocolTCP, metricsPort)
	RequireIngressFromNamespace(t, policy, metricsPort, openshiftMonitoringNamespace)
	RequireIngressFromNamespace(t, policy, metricsPort, openshiftUWMNamespace)
	RequireIngressFromNamespaceLabel(t, policy, metricsPort, clusterMonitoringLabel, "true")
	t.Logf(" - Ingress: TCP/%d from monitoring namespaces", metricsPort)

	RequireIngressPort(t, policy, corev1.ProtocolTCP, pluginPort)
	RequireIngressFromPolicyGroup(t, policy, pluginPort, ingressPolicyGroupKey)
	t.Logf(" - Ingress: TCP/%d from %s", pluginPort, ingressPolicyGroupKey)

	if HasPortInIngress(policy.Spec.Ingress, corev1.ProtocolTCP, healthPort) {
		t.Fatalf("%s/%s: port %d must not be allowed by ingress rules", policy.Namespace, policy.Name, healthPort)
	}
	t.Logf(" - Ingress: TCP/%d is not allowed (health/serving)", healthPort)

	RequireUnrestrictedEgress(t, policy)
	t.Logf(" - Egress: unrestricted [{}]")

	o.Expect(policy.Spec.PolicyTypes).To(o.ContainElement(networkingv1.PolicyTypeIngress))
	o.Expect(policy.Spec.PolicyTypes).To(o.ContainElement(networkingv1.PolicyTypeEgress))

	RequireOwnerReference(t, policy, "operator.openshift.io/v1", "CliManager", operatorclient.OperatorConfigName)
	t.Logf(" - OwnerReference: CliManager/%s", operatorclient.OperatorConfigName)

	t.Logf("=== operand NetworkPolicy validated ===")
}

func testOperandMetricsIngress(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	leader := waitForLeaderOperandPod(t, ctx, kubeClient)
	leaderIPs := PodIPs(leader)
	testLabels := map[string]string{"test": "cli-manager-netpol"}

	t.Logf("=== Testing metrics ingress on port %d to leader %s ===", metricsPort, leader.Name)

	t.Logf("3.1 Allowed — Ingress from %s on port %d", openshiftMonitoringNamespace, metricsPort)
	ExpectConnectivity(ctx, t, kubeClient, openshiftMonitoringNamespace, testLabels, leaderIPs, metricsPort, true)

	if namespaceExists(ctx, kubeClient, openshiftUWMNamespace) {
		t.Logf("3.2 Allowed — Ingress from %s on port %d", openshiftUWMNamespace, metricsPort)
		ExpectConnectivity(ctx, t, kubeClient, openshiftUWMNamespace, testLabels, leaderIPs, metricsPort, true)
	} else {
		t.Logf("3.2 Skipping %s; namespace not present", openshiftUWMNamespace)
	}

	t.Logf("3.3 Blocked — Ingress from default namespace on port %d", metricsPort)
	ExpectConnectivity(ctx, t, kubeClient, "default", testLabels, leaderIPs, metricsPort, false)

	t.Logf("3.4 Blocked — Ingress from %s on unused port %d", openshiftMonitoringNamespace, unusedPort)
	ExpectConnectivity(ctx, t, kubeClient, openshiftMonitoringNamespace, testLabels, leaderIPs, unusedPort, false)

	t.Logf("3.5 Allowed — Ingress from operator namespace on port %d (cluster-monitoring label)", metricsPort)
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, testLabels, leaderIPs, metricsPort, true)

	t.Logf("=== metrics ingress verified ===")
}

func testOperandPluginIngress(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	leader := waitForLeaderOperandPod(t, ctx, kubeClient)
	leaderIPs := PodIPs(leader)
	testLabels := map[string]string{"test": "cli-manager-netpol"}

	// Use a temp namespace with the ingress policy-group label instead of
	// openshift-ingress directly — that namespace has deny-all policies
	// blocking non-router pods on OCP 5.x+.
	ingressNS, cleanup := createTempIngressNamespace(t, ctx, kubeClient)
	defer cleanup()

	t.Logf("=== Testing plugin download ingress on port %d to leader %s ===", pluginPort, leader.Name)

	t.Logf("3.6 Allowed — Ingress from %s (policy-group ingress label) on port %d", ingressNS, pluginPort)
	ExpectConnectivity(ctx, t, kubeClient, ingressNS, testLabels, leaderIPs, pluginPort, true)

	t.Logf("3.7 Blocked — Ingress from default namespace on port %d", pluginPort)
	ExpectConnectivity(ctx, t, kubeClient, "default", testLabels, leaderIPs, pluginPort, false)

	t.Logf("3.10 Blocked — Ingress from operator namespace on port %d (no ingress policy-group label)", pluginPort)
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, testLabels, leaderIPs, pluginPort, false)

	if ingressNamespaceHasDenyAll(ctx, kubeClient) {
		t.Logf("3.11 Blocked — Non-router pod in %s blocked by %s deny-all on port %d",
			openshiftIngressNamespace, openshiftIngressDenyAllPolicy, pluginPort)
		ExpectConnectivity(ctx, t, kubeClient, openshiftIngressNamespace, testLabels, leaderIPs, pluginPort, false)
	} else {
		t.Logf("3.11 Skipping — %s/%s not present, non-router pods can egress",
			openshiftIngressNamespace, openshiftIngressDenyAllPolicy)
	}

	t.Logf("=== plugin download ingress verified ===")
}

func testOperandHealthAndHostNetworkIngress(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	leader := waitForLeaderOperandPod(t, ctx, kubeClient)
	leaderIPs := PodIPs(leader)
	testLabels := map[string]string{"test": "cli-manager-netpol"}

	t.Logf("=== Testing health ingress and host-network bypass ===")

	t.Logf("3.9 Blocked — Ingress from %s on port %d", openshiftMonitoringNamespace, healthPort)
	ExpectConnectivity(ctx, t, kubeClient, openshiftMonitoringNamespace, testLabels, leaderIPs, healthPort, false)

	t.Logf("3.8 Allowed — Host network (kubelet path) to port %d", metricsPort)
	ExpectHostNetworkConnectivity(ctx, t, kubeClient, openshiftMonitoringNamespace, leader.Spec.NodeName, leaderIPs, metricsPort, true)

	t.Logf("=== health and host-network ingress verified ===")
}

func testOperandEgress(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	clientLabels := operandClientLabels()

	t.Logf("=== Testing operand egress (unrestricted [{}]) ===")

	dnsSvc, err := kubeClient.CoreV1().Services(openshiftDNSNamespace).Get(ctx, "dns-default", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should get dns-default service")
	t.Logf("4.1 Allowed — DNS resolution via %s:53", dnsSvc.Spec.ClusterIP)
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, clientLabels, ServiceClusterIPs(dnsSvc), 53, true)

	kubeSvc, err := kubeClient.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should get kubernetes service")
	t.Logf("4.2 Allowed — API server connectivity on 443")
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, clientLabels, ServiceClusterIPs(kubeSvc), 443, true)

	t.Logf("4.3 Allowed — External internet (1.1.1.1:443)")
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, clientLabels, []string{"1.1.1.1"}, 443, true)

	promSvc, err := kubeClient.CoreV1().Services(openshiftMonitoringNamespace).Get(ctx, prometheusK8sServiceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should get prometheus-k8s service")
	promPort := int32(9091)
	for _, p := range promSvc.Spec.Ports {
		if p.Name == "web" || p.Port == 9091 {
			promPort = p.Port
			break
		}
	}
	t.Logf("4.4 Allowed — Cross-namespace egress to prometheus-k8s:%d", promPort)
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, clientLabels, ServiceClusterIPs(promSvc), promPort, true)

	t.Logf("=== operand egress verified ===")
}

func testOperandServiceMonitor(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	t.Logf("=== Validating ServiceMonitor and metrics service ===")

	dynamicClient := GetApiDynamicClient()
	gvr := schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}
	obj, err := dynamicClient.Resource(gvr).Namespace(operatorclient.OperatorNamespace).Get(ctx, operandServiceMonitorName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "ServiceMonitor %s should exist", operandServiceMonitorName)

	labels, found, err := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue(), "ServiceMonitor should have a selector")
	o.Expect(labels[operandAppLabelKey]).To(o.Equal(operatorclient.OperandName))

	endpoints, found, err := unstructured.NestedSlice(obj.Object, "spec", "endpoints")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())
	o.Expect(endpoints).NotTo(o.BeEmpty())
	endpoint, ok := endpoints[0].(map[string]any)
	o.Expect(ok).To(o.BeTrue(), "ServiceMonitor endpoint should be an object")

	port, found, err := unstructured.NestedString(endpoint, "port")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())
	o.Expect(port).To(o.Equal(operandMetricsPortName))

	scheme, found, err := unstructured.NestedString(endpoint, "scheme")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(found).To(o.BeTrue())
	o.Expect(scheme).To(o.Equal("https"))
	t.Logf(" - ServiceMonitor port=%s scheme=%s selector=%s=%s", port, scheme, operandAppLabelKey, labels[operandAppLabelKey])

	metricsSvc, err := kubeClient.CoreV1().Services(operatorclient.OperatorNamespace).Get(ctx, operandMetricsServiceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "metrics service should exist")
	o.Expect(metricsSvc.Spec.ClusterIP).To(o.Equal(corev1.ClusterIPNone), "metrics service should be headless")
	o.Expect(metricsSvc.Spec.Ports).NotTo(o.BeEmpty())
	o.Expect(metricsSvc.Spec.Ports[0].Port).To(o.Equal(metricsPort))
	t.Logf(" - Service %s is headless on port %d", operandMetricsServiceName, metricsPort)

	pluginSvc, err := kubeClient.CoreV1().Services(operatorclient.OperatorNamespace).Get(ctx, operandPluginServiceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "plugin service should exist")
	o.Expect(pluginSvc.Spec.Ports).NotTo(o.BeEmpty())
	o.Expect(pluginSvc.Spec.Ports[0].Port).To(o.Equal(pluginPort))
	t.Logf(" - Service %s on port %d", operandPluginServiceName, pluginPort)

	t.Logf("=== ServiceMonitor configuration verified ===")
}

func testUnlabeledPodEgress(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	dnsSvc, err := kubeClient.CoreV1().Services(openshiftDNSNamespace).Get(ctx, "dns-default", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	dnsIPs := ServiceClusterIPs(dnsSvc)

	t.Logf("=== Testing unlabeled vs labeled pod egress ===")

	t.Logf("8.1 Unlabeled pod can reach DNS (no default-deny policy)")
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, map[string]string{"test": "unlabeled"}, dnsIPs, 53, true)

	t.Logf("8.2 Labeled operand pod can reach DNS via unrestricted egress")
	ExpectConnectivity(ctx, t, kubeClient, operatorclient.OperatorNamespace, operandClientLabels(), dnsIPs, 53, true)

	t.Logf("=== unlabeled/labeled egress verified ===")
}

func testCustomNetworkPolicyPreserved(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	const customName = "test-custom-np"
	t.Logf("=== Creating unmanaged NetworkPolicy %s/%s ===", operatorclient.OperatorNamespace, customName)

	custom := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      customName,
			Namespace: operatorclient.OperatorNamespace,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{"test": "true"},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test-app"}}},
					},
				},
			},
		},
	}
	_, err := kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Create(ctx, custom, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should create unmanaged custom NetworkPolicy")
	defer func() {
		_ = kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Delete(ctx, customName, metav1.DeleteOptions{})
	}()

	AssertUnmanagedNetworkPolicyPreserved(t, ctx, kubeClient, operatorclient.OperatorNamespace, customName, 30*time.Second)
	t.Logf("=== unmanaged custom NetworkPolicy preserved ===")
}

func testOperandNetworkPolicyReconciliation(t testing.TB, ctx context.Context, kubeClient k8sclient.Interface) {
	t.Helper()
	t.Logf("=== Testing operand NetworkPolicy reconciliation ===")

	t.Logf("7.1 Mutate ingress port and wait for revert")
	MutatePortAndRestoreNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName, networkPolicyReconcileTimeout)

	t.Logf("7.2 Mutate pod selector and wait for revert")
	MutateAndRestoreNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName, networkPolicyReconcileTimeout)

	t.Logf("7.4 Mutate policyTypes and wait for revert")
	MutatePolicyTypesAndRestoreNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName, networkPolicyReconcileTimeout)

	t.Logf("7.5 Mutate monitoring namespaceSelector and wait for revert")
	MutateNamespaceSelectorAndRestoreNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName, networkPolicyReconcileTimeout)

	t.Logf("7.6 Clear ingress rules and wait for restore")
	MutateEmptyIngressAndRestoreNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName, networkPolicyReconcileTimeout)

	t.Logf("7.3 Delete policy and wait for recreate")
	expected := GetNetworkPolicy(t, ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)
	RestoreNetworkPolicy(t, ctx, kubeClient, expected, networkPolicyReconcileTimeout)

	LogNetworkPolicyEvents(t, ctx, kubeClient, []string{operatorclient.OperatorNamespace}, operandNetworkPolicyName)
	t.Logf("=== operand NetworkPolicy reconciliation verified ===")
}

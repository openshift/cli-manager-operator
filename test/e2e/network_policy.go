package e2e

// NetworkPolicy e2e coverage for the operand policy
// allow-all-egress-and-metrics-ingress-operand.
//
// Test Scenarios:
//
//	Policy Definition & Spec - Validates NetworkPolicy structure, selectors, and egress rules
//	Ingress Policy Enforcement - Tests metrics, plugin, and health port access control
//	Host Network Bypass - Verifies kubelet/host-network traffic bypasses NetworkPolicy
//	Egress Connectivity - Tests unrestricted egress (DNS, API, external, prometheus)
//	Policy Preservation - Custom unmanaged policies are not pruned/deleted
//	Mutation Recovery & Reconciliation - Policy auto-restores after mutations and deletion

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"

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
		waitForOperandNetworkPolicy(ctx, kubeClient)
		waitForLeaderOperandPod(ctx, kubeClient)
	})

	g.AfterAll(func() {
		if cancelFnc != nil {
			cancelFnc()
		}
	})

	// Scenario 1: Policy Definition & Spec Validation
	g.It("should define operand NetworkPolicy with correct structure and selectors", func() {
		testPolicyDefinition(ctx, kubeClient)
	})

	// Scenario 2: Ingress Policy Enforcement (Metrics, Plugin, Health)
	g.It("should enforce ingress policy for metrics, plugin, and health ports", func() {
		testIngressPolicyEnforcement(ctx, kubeClient)
	})

	// Scenario 3: Host Network Bypass
	g.It(" should allow kubelet/host-network to bypass NetworkPolicy", func() {
		testHostNetworkBypass(ctx, kubeClient)
	})

	// Scenario 4: Egress Connectivity
	g.It("should allow unrestricted egress for DNS, API, external, and prometheus", func() {
		testEgressConnectivity(ctx, kubeClient)
	})

	// Scenario 5: Policy Preservation
	g.It("should preserve custom unmanaged policies and not prune them", func() {
		testPolicyPreservation(ctx, kubeClient)
	})

	// Scenario 6: Mutation Recovery & Reconciliation
	g.It("should reconcile policy mutations and restore deleted policies", func() {
		testMutationRecoveryAndReconciliation(ctx, kubeClient)
	})
})

func testPolicyDefinition(ctx context.Context, kubeClient k8sclient.Interface) {
	g.By("Fetching operand NetworkPolicy")
	policy := GetNetworkPolicy(ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)
	g.By(fmt.Sprintf("Policy found: %s/%s", policy.Namespace, policy.Name))

	g.By("Validating pod selector")
	RequirePodSelectorLabel(policy, operandAppLabelKey, operatorclient.OperandName)

	g.By("Validating ingress port configuration")
	RequireIngressPort(policy, corev1.ProtocolTCP, metricsPort)
	RequireIngressPort(policy, corev1.ProtocolTCP, pluginPort)

	// health port should NOT be in ingress rules
	healthPortFound := false
	for _, rule := range policy.Spec.Ingress {
		for _, p := range rule.Ports {
			if p.Protocol != nil && *p.Protocol != corev1.ProtocolTCP {
				continue
			}
			if p.Port == nil || p.Port.IntValue() == int(healthPort) {
				healthPortFound = true
				break
			}
		}
		if healthPortFound {
			break
		}
	}
	o.Expect(healthPortFound).To(o.BeFalse(), "health port should not be allowed")

	g.By("Validating ingress namespace sources")
	RequireIngressFromNamespaceLabel(policy, metricsPort, "kubernetes.io/metadata.name", openshiftMonitoringNamespace)
	RequireIngressFromNamespaceLabel(policy, metricsPort, clusterMonitoringLabel, "true")
	RequireIngressFromPolicyGroup(policy, pluginPort, ingressPolicyGroupKey)

	g.By("Validating egress configuration")
	RequireUnrestrictedEgress(policy)

	g.By("Validating policy types")
	o.Expect(policy.Spec.PolicyTypes).To(o.ContainElement(networkingv1.PolicyTypeIngress))
	o.Expect(policy.Spec.PolicyTypes).To(o.ContainElement(networkingv1.PolicyTypeEgress))

	g.By("Validating owner reference")
	RequireOwnerReference(policy, "operator.openshift.io/v1", "CliManager", operatorclient.OperatorConfigName)

	g.By("Policy definition validated ✓")
}

type ingressTestCase struct {
	description string
	namespace   string
	port        int32
	shouldAllow bool
	skipIfNoNs  bool // skip test if namespace doesn't exist
}

func testIngressPolicyEnforcement(ctx context.Context, kubeClient k8sclient.Interface) {
	leader := waitForLeaderOperandPod(ctx, kubeClient)
	leaderIPs := []string{leader.Status.PodIP}
	testLabels := map[string]string{"test": "cli-manager-netpol"}

	ingressTests := []ingressTestCase{
		// Metrics port (60000) - Should be accessible from monitoring namespaces
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
			skipIfNoNs:  true,
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

		// Plugin port (9449) - Should be accessible from ingress policy-group only
		{
			description: "plugin port blocked from default namespace",
			namespace:   "default",
			port:        pluginPort,
			shouldAllow: false,
		},
		// Health port (8443) - Should never be accessible via NetworkPolicy
		{
			description: "health port blocked from monitoring namespace",
			namespace:   openshiftMonitoringNamespace,
			port:        healthPort,
			shouldAllow: false,
		},
	}

	g.By("Testing ingress policy enforcement")
	for _, tc := range ingressTests {
		if tc.skipIfNoNs && !namespaceExists(ctx, kubeClient, tc.namespace) {
			g.By(fmt.Sprintf("SKIP: %s (namespace doesn't exist)", tc.description))
			continue
		}

		g.By(fmt.Sprintf("%s", tc.description))
		ExpectConnectivity(ctx, kubeClient, tc.namespace, testLabels, leaderIPs, tc.port, tc.shouldAllow)
	}

	g.By("Ingress policy enforcement validated ✓")
}

func testHostNetworkBypass(ctx context.Context, kubeClient k8sclient.Interface) {
	leader := waitForLeaderOperandPod(ctx, kubeClient)
	leaderIPs := []string{leader.Status.PodIP}

	g.By(" Kubelet/host-network should bypass metrics port policy")
	ExpectHostNetworkConnectivity(ctx, kubeClient, openshiftMonitoringNamespace, leader.Spec.NodeName, leaderIPs, metricsPort, true)

	g.By(" Host network bypass validated ✓")
}

func testEgressConnectivity(ctx context.Context, kubeClient k8sclient.Interface) {
	clientLabels := operandClientLabels()

	g.By("Testing egress connectivity")

	// DNS
	dnsSvc, err := kubeClient.CoreV1().Services(openshiftDNSNamespace).Get(ctx, "dns-default", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	dnsIPs := ServiceClusterIPs(dnsSvc)

	g.By("DNS: Allowed to openshiftDNSNamespace/dns-default:53")
	ExpectConnectivity(ctx, kubeClient, operatorclient.OperatorNamespace, clientLabels, dnsIPs, 53, true)

	// Kubernetes API
	kubeSvc, err := kubeClient.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	kubeIPs := ServiceClusterIPs(kubeSvc)

	g.By("Kubernetes API: Allowed to default/kubernetes:443")
	ExpectConnectivity(ctx, kubeClient, operatorclient.OperatorNamespace, clientLabels, kubeIPs, 443, true)

	// Prometheus
	promSvc, err := kubeClient.CoreV1().Services(openshiftMonitoringNamespace).Get(ctx, prometheusK8sServiceName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	promPort := int32(9091)
	for _, p := range promSvc.Spec.Ports {
		if p.Name == "web" || p.Port == 9091 {
			promPort = p.Port
			break
		}
	}
	promIPs := ServiceClusterIPs(promSvc)

	g.By(fmt.Sprintf("Prometheus: Allowed to prometheus-k8s:%d", promPort))
	ExpectConnectivity(ctx, kubeClient, operatorclient.OperatorNamespace, clientLabels, promIPs, promPort, true)

	g.By("Egress connectivity validated ✓")
}

func testPolicyPreservation(ctx context.Context, kubeClient k8sclient.Interface) {
	g.By("Testing policy preservation for custom unmanaged policies")

	g.By("Creating custom unmanaged NetworkPolicy")
	const customName = "test-custom-np"

	// Load custom NetworkPolicy from embedded YAML manifest
	npYAML, err := testNetPolicyData.ReadFile("testdata/custom-netpolicy.yaml")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to load custom netpolicy manifest")

	// Replace template variables
	npYAML = []byte(strings.ReplaceAll(string(npYAML), "{{NAMESPACE}}", operatorclient.OperatorNamespace))

	// Unmarshal YAML to NetworkPolicy object
	custom := &networkingv1.NetworkPolicy{}
	if err := yaml.Unmarshal(npYAML, custom); err != nil {
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to unmarshal custom netpolicy manifest")
	}

	_, err = kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Create(ctx, custom, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	defer func() {
		_ = kubeClient.NetworkingV1().NetworkPolicies(operatorclient.OperatorNamespace).Delete(ctx, customName, metav1.DeleteOptions{})
	}()

	g.By("Verifying custom policy is not pruned by the operator")
	AssertUnmanagedNetworkPolicyPreserved(ctx, kubeClient, operatorclient.OperatorNamespace, customName, 30*time.Second)

	g.By("Policy preservation validated ✓")
}

func testMutationRecoveryAndReconciliation(ctx context.Context, kubeClient k8sclient.Interface) {
	g.By("Testing mutation recovery and operator reconciliation")

	expected := GetNetworkPolicy(ctx, kubeClient, operatorclient.OperatorNamespace, operandNetworkPolicyName)

	mutations := []struct {
		description string
		fn          func()
	}{
		{
			description: "port mutation",
			fn: func() {
				patch := []byte(`[{"op":"replace","path":"/spec/ingress/0/ports/0/port","value":9999}]`)
				testMutationRecovery(ctx, kubeClient, patch, networkPolicyReconcileTimeout)
			},
		},
	}

	g.By("Testing reconciliation of individual mutations")
	for _, m := range mutations {
		g.By(fmt.Sprintf("Testing %s recovery", m.description))
		m.fn()
	}

	g.By("Testing deletion and recreation of policy")
	RestoreNetworkPolicy(ctx, kubeClient, expected, networkPolicyReconcileTimeout)

	LogNetworkPolicyEvents(ctx, kubeClient, []string{operatorclient.OperatorNamespace}, operandNetworkPolicyName)

	g.By("Mutation recovery and reconciliation validated ✓")
}

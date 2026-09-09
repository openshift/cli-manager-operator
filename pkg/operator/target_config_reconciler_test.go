package operator

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"

	climanagerv1 "github.com/openshift/cli-manager-operator/pkg/apis/climanager/v1"
	"github.com/openshift/cli-manager-operator/pkg/operator/operatorclient"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	allowNetworkPolicyOperandName = "allow-all-egress-and-metrics-ingress-operand"
)

func TestManageOperandNetworkPolicyAllow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kubeClient := fake.NewSimpleClientset()
	eventRecorder := events.NewInMemoryRecorder("test", clock.RealClock{})

	cliManager := &climanagerv1.CliManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorclient.OperatorConfigName,
			Namespace: operatorclient.OperatorNamespace,
			UID:       "test-uid",
		},
	}

	reconciler := &TargetConfigReconciler{
		ctx:           ctx,
		kubeClient:    kubeClient,
		eventRecorder: eventRecorder,
	}

	policy, modified, err := reconciler.manageOperandNetworkPolicyAllow(cliManager)
	if err != nil {
		t.Fatalf("manageOperandNetworkPolicyAllow failed: %v", err)
	}

	if !modified {
		t.Error("Expected modified=true when creating policy")
	}

	if policy.GetName() != allowNetworkPolicyOperandName {
		t.Errorf("Expected policy name %q, got %q", allowNetworkPolicyOperandName, policy.GetName())
	}

	if policy.GetNamespace() != operatorclient.OperatorNamespace {
		t.Errorf("Expected policy namespace %q, got %q", operatorclient.OperatorNamespace, policy.GetNamespace())
	}
}

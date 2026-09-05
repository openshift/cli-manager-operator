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
	allowNetworkPolicyOperandName       = "allow-all-egress-and-metrics-ingress-operand"
	defaultDenyNetworkPolicyOperandName = "default-deny-operand"
)

// verifyNetworkPolicy checks that the network policy has the expected name and namespace
func verifyNetworkPolicy(t *testing.T, obj metav1.Object, expectedName string) {
	t.Helper()

	if obj.GetName() != expectedName {
		t.Errorf("Expected policy name %q, got %q", expectedName, obj.GetName())
	}

	if obj.GetNamespace() != operatorclient.OperatorNamespace {
		t.Errorf("Expected policy namespace %q, got %q", operatorclient.OperatorNamespace, obj.GetNamespace())
	}
}

func TestManageOperandNetworkPolicies(t *testing.T) {
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

	tests := []struct {
		name         string
		manageFunc   func(*TargetConfigReconciler, *climanagerv1.CliManager) (metav1.Object, bool, error)
		expectedName string
	}{
		{
			name: "creates allow network policy",
			manageFunc: func(r *TargetConfigReconciler, cm *climanagerv1.CliManager) (metav1.Object, bool, error) {
				obj, modified, err := r.manageOperandNetworkPolicyAllow(cm)
				return obj, modified, err
			},
			expectedName: allowNetworkPolicyOperandName,
		},
		{
			name: "creates default deny network policy",
			manageFunc: func(r *TargetConfigReconciler, cm *climanagerv1.CliManager) (metav1.Object, bool, error) {
				obj, modified, err := r.manageOperandDefaultDenyNetworkPolicy(cm)
				return obj, modified, err
			},
			expectedName: defaultDenyNetworkPolicyOperandName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj, modified, err := tt.manageFunc(reconciler, cliManager)
			if err != nil {
				t.Fatalf("manage function failed: %v", err)
			}

			if !modified {
				t.Error("Expected modified=true when creating policy")
			}

			verifyNetworkPolicy(t, obj, tt.expectedName)
		})
	}
}

func TestCheckNetworkPolicyExists(t *testing.T) {
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

	t.Run("returns false for non-existent policy", func(t *testing.T) {
		exists := reconciler.checkNetworkPolicyExists("non-existent-ns", "non-existent-policy")
		if exists {
			t.Error("Expected checkNetworkPolicyExists to return false for non-existent policy")
		}
	})

	t.Run("returns true after creating policy", func(t *testing.T) {
		// Create a policy explicitly for this test
		_, _, err := reconciler.manageOperandNetworkPolicyAllow(cliManager)
		if err != nil {
			t.Fatalf("failed to create network policy: %v", err)
		}

		// After creating a policy, it should return true
		exists := reconciler.checkNetworkPolicyExists(operatorclient.OperatorNamespace, allowNetworkPolicyOperandName)
		if !exists {
			t.Error("Expected checkNetworkPolicyExists to return true for created policy")
		}
	})
}

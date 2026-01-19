package e2e

import (
	"os"
	"testing"

	o "github.com/onsi/gomega"
	"k8s.io/klog/v2"
)

// NOTE: This test is also available in the OTE framework (test/e2e/operator.go).
// This dual implementation allows tests to run both as standard Go tests (via go test)
// and through the Ginkgo/OTE framework (for OpenShift CI integration).
//
// The actual test logic is in operator.go's standalone functions, which are called
// by both this standard Go test and the Ginkgo specs.

func TestMain(m *testing.M) {
	if os.Getenv("KUBECONFIG") == "" {
		klog.Errorf("KUBECONFIG environment variable not set")
		os.Exit(1)
	}

	if os.Getenv("RELEASE_IMAGE_LATEST") == "" {
		klog.Errorf("RELEASE_IMAGE_LATEST environment variable not set")
		os.Exit(1)
	}

	if os.Getenv("NAMESPACE") == "" {
		klog.Errorf("NAMESPACE environment variable not set")
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// TestExtended runs the operator tests using standard Go testing.
func TestExtended(t *testing.T) {
	// Register Gomega with the testing framework for standard Go test mode
	o.RegisterTestingT(t)

	t.Run("CLI Manager Operator", func(t *testing.T) {
		// Setup operator and wait for it to be ready
		ctx, cancelFnc, kubeClient, err := setupOperator(t)
		if err != nil {
			t.Fatalf("Failed to setup operator: %v", err)
		}
		defer cancelFnc()

		t.Run("CLI Manager functionality", func(t *testing.T) {
			testCLIManager(t, ctx, kubeClient)
		})
	})
}

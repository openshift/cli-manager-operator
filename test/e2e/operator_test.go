package e2e

import (
	"os"
	"testing"

	"k8s.io/klog/v2"
)

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

func TestCLIManager(t *testing.T) {
	// Set up the operator environment
	setupOperator(t)

	// Run the CLI Manager tests
	testCLIManager(t)
}

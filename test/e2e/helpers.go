package e2e

import (
	"os"

	o "github.com/onsi/gomega"
	apiextclientv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	climanagerclient "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
)

// GetKubeClient returns a Kubernetes clientset.
func GetKubeClient() *k8sclient.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred())

	client, err := k8sclient.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred())

	return client
}

// GetApiExtensionClient returns an API extensions clientset for CRD operations.
func GetApiExtensionClient() *apiextclientv1.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred())

	client, err := apiextclientv1.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred())

	return client
}

// GetApiDynamicClient returns a dynamic Kubernetes client.
func GetApiDynamicClient() *dynamic.DynamicClient {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred())

	client, err := dynamic.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred())

	return client
}

// GetCLIManagerClient returns a CLIManager clientset for operator-specific resources.
func GetCLIManagerClient() *climanagerclient.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred())

	client, err := climanagerclient.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred())

	return client
}

// GetRouteClient returns an OpenShift route client.
func GetRouteClient() routev1client.RoutesGetter {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred())

	client, err := routev1client.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred())

	return client
}

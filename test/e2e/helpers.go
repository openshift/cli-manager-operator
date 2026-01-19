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

// GetKubeClient returns a Kubernetes clientset or fails the test
func GetKubeClient() *k8sclient.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred(), "should build kubeconfig")

	client, err := k8sclient.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create kubernetes client")

	return client
}

// GetApiExtensionClient returns an API extensions clientset or fails the test
func GetApiExtensionClient() *apiextclientv1.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred(), "should build kubeconfig")

	client, err := apiextclientv1.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create API extension client")

	return client
}

// GetApiDynamicClient returns a dynamic Kubernetes client or fails the test
func GetApiDynamicClient() *dynamic.DynamicClient {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred(), "should build kubeconfig")

	client, err := dynamic.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create dynamic client")

	return client
}

// GetCLIManagerClient returns a CLIManager clientset or fails the test
func GetCLIManagerClient() *climanagerclient.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred(), "should build kubeconfig")

	client, err := climanagerclient.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create CLIManager client")

	return client
}

// GetRouteClient returns an OpenShift route client or fails the test
func GetRouteClient() routev1client.RoutesGetter {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	o.Expect(err).NotTo(o.HaveOccurred(), "should build kubeconfig")

	client, err := routev1client.NewForConfig(config)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create route client")

	return client
}

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1 "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/typed/climanager/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeClimanagersV1 struct {
	*testing.Fake
}

func (c *FakeClimanagersV1) CLIManagers(namespace string) v1.CLIManagerInterface {
	return &FakeCLIManagers{c, namespace}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeClimanagersV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}

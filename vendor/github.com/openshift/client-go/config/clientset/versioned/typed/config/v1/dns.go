// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	context "context"

	configv1 "github.com/openshift/api/config/v1"
	applyconfigurationsconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	scheme "github.com/openshift/client-go/config/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	gentype "k8s.io/client-go/gentype"
)

// DNSesGetter has a method to return a DNSInterface.
// A group's client should implement this interface.
type DNSesGetter interface {
	DNSes() DNSInterface
}

// DNSInterface has methods to work with DNS resources.
type DNSInterface interface {
	Create(ctx context.Context, dNS *configv1.DNS, opts metav1.CreateOptions) (*configv1.DNS, error)
	Update(ctx context.Context, dNS *configv1.DNS, opts metav1.UpdateOptions) (*configv1.DNS, error)
	// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
	UpdateStatus(ctx context.Context, dNS *configv1.DNS, opts metav1.UpdateOptions) (*configv1.DNS, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*configv1.DNS, error)
	List(ctx context.Context, opts metav1.ListOptions) (*configv1.DNSList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *configv1.DNS, err error)
	Apply(ctx context.Context, dNS *applyconfigurationsconfigv1.DNSApplyConfiguration, opts metav1.ApplyOptions) (result *configv1.DNS, err error)
	// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
	ApplyStatus(ctx context.Context, dNS *applyconfigurationsconfigv1.DNSApplyConfiguration, opts metav1.ApplyOptions) (result *configv1.DNS, err error)
	DNSExpansion
}

// dNSes implements DNSInterface
type dNSes struct {
	*gentype.ClientWithListAndApply[*configv1.DNS, *configv1.DNSList, *applyconfigurationsconfigv1.DNSApplyConfiguration]
}

// newDNSes returns a DNSes
func newDNSes(c *ConfigV1Client) *dNSes {
	return &dNSes{
		gentype.NewClientWithListAndApply[*configv1.DNS, *configv1.DNSList, *applyconfigurationsconfigv1.DNSApplyConfiguration](
			"dnses",
			c.RESTClient(),
			scheme.ParameterCodec,
			"",
			func() *configv1.DNS { return &configv1.DNS{} },
			func() *configv1.DNSList { return &configv1.DNSList{} },
		),
	}
}

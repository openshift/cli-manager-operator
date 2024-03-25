package operatorclient

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
	operatorconfigclientv1 "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/typed/climanager/v1"
)

const OperatorNamespace = "openshift-cli-manager-operator"
const OperatorConfigName = "cluster"
const OperandName = "cli-manager"

type CLIManagerClient struct {
	Ctx            context.Context
	SharedInformer cache.SharedIndexInformer
	OperatorClient operatorconfigclientv1.ClimanagersV1Interface
}

func (c *CLIManagerClient) Informer() cache.SharedIndexInformer {
	return c.SharedInformer
}

func (c *CLIManagerClient) GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error) {
	instance, err := c.OperatorClient.CLIManagers(OperatorNamespace).Get(c.Ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, "", err
	}
	return &instance.Spec.OperatorSpec, &instance.Status.OperatorStatus, instance.ResourceVersion, nil
}

func (c CLIManagerClient) GetOperatorStateWithQuorum(ctx context.Context) (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	instance, err := c.OperatorClient.CLIManagers(OperatorNamespace).Get(ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, "", err
	}

	return &instance.Spec.OperatorSpec, &instance.Status.OperatorStatus, instance.GetResourceVersion(), nil
}

func (c *CLIManagerClient) UpdateOperatorSpec(ctx context.Context, resourceVersion string, spec *operatorv1.OperatorSpec) (out *operatorv1.OperatorSpec, newResourceVersion string, err error) {
	original, err := c.OperatorClient.CLIManagers(OperatorNamespace).Get(ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, "", err
	}
	copy := original.DeepCopy()
	copy.ResourceVersion = resourceVersion
	copy.Spec.OperatorSpec = *spec

	ret, err := c.OperatorClient.CLIManagers(OperatorNamespace).Update(ctx, copy, v1.UpdateOptions{})
	if err != nil {
		return nil, "", err
	}

	return &ret.Spec.OperatorSpec, ret.ResourceVersion, nil
}

func (c *CLIManagerClient) UpdateOperatorStatus(ctx context.Context, resourceVersion string, status *operatorv1.OperatorStatus) (out *operatorv1.OperatorStatus, err error) {
	original, err := c.OperatorClient.CLIManagers(OperatorNamespace).Get(ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	copy := original.DeepCopy()
	copy.ResourceVersion = resourceVersion
	copy.Status.OperatorStatus = *status

	ret, err := c.OperatorClient.CLIManagers(OperatorNamespace).UpdateStatus(ctx, copy, v1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return &ret.Status.OperatorStatus, nil
}

func (c *CLIManagerClient) GetObjectMeta() (meta *metav1.ObjectMeta, err error) {
	instance, err := c.OperatorClient.CLIManagers(OperatorNamespace).Get(c.Ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &instance.ObjectMeta, nil
}

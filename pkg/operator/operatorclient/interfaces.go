package operatorclient

import (
	"context"
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"
	climanagerapplyconfiguration "github.com/openshift/cli-manager-operator/pkg/generated/applyconfiguration/climanager/v1"
	operatorconfigclientv1 "github.com/openshift/cli-manager-operator/pkg/generated/clientset/versioned/typed/climanager/v1"
	applyconfiguration "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
)

const OperatorNamespace = "openshift-cli-manager-operator"
const OperatorConfigName = "cluster"
const OperandName = "openshift-cli-manager"

type CLIManagerClient struct {
	Ctx            context.Context
	SharedInformer cache.SharedIndexInformer
	OperatorClient operatorconfigclientv1.ClimanagersV1Interface
}

func (c *CLIManagerClient) Informer() cache.SharedIndexInformer {
	return c.SharedInformer
}

func (c *CLIManagerClient) GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error) {
	instance, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(c.Ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, "", err
	}
	return &instance.Spec.OperatorSpec, &instance.Status.OperatorStatus, instance.ResourceVersion, nil
}

func (c *CLIManagerClient) GetOperatorStateWithQuorum(ctx context.Context) (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, string, error) {
	instance, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, "", err
	}

	return &instance.Spec.OperatorSpec, &instance.Status.OperatorStatus, instance.GetResourceVersion(), nil
}

func (c *CLIManagerClient) UpdateOperatorSpec(ctx context.Context, resourceVersion string, spec *operatorv1.OperatorSpec) (out *operatorv1.OperatorSpec, newResourceVersion string, err error) {
	original, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, "", err
	}
	copy := original.DeepCopy()
	copy.ResourceVersion = resourceVersion
	copy.Spec.OperatorSpec = *spec

	ret, err := c.OperatorClient.CliManagers(OperatorNamespace).Update(ctx, copy, v1.UpdateOptions{})
	if err != nil {
		return nil, "", err
	}

	return &ret.Spec.OperatorSpec, ret.ResourceVersion, nil
}

func (c *CLIManagerClient) UpdateOperatorStatus(ctx context.Context, resourceVersion string, status *operatorv1.OperatorStatus) (out *operatorv1.OperatorStatus, err error) {
	original, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	copy := original.DeepCopy()
	copy.ResourceVersion = resourceVersion
	copy.Status.OperatorStatus = *status

	ret, err := c.OperatorClient.CliManagers(OperatorNamespace).UpdateStatus(ctx, copy, v1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return &ret.Status.OperatorStatus, nil
}

func (c *CLIManagerClient) GetObjectMeta() (meta *metav1.ObjectMeta, err error) {
	instance, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(c.Ctx, OperatorConfigName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &instance.ObjectMeta, nil
}

func (c *CLIManagerClient) ApplyOperatorSpec(ctx context.Context, fieldManager string, desiredConfiguration *applyconfiguration.OperatorSpecApplyConfiguration) error {
	if desiredConfiguration == nil {
		return fmt.Errorf("applyConfiguration must have a value")
	}

	jsonBytes, err := json.Marshal(desiredConfiguration)
	if err != nil {
		return fmt.Errorf("unable to serialize operator configuration: %w", err)
	}
	operatorSpec := &operatorv1.OperatorSpec{}
	if err := json.Unmarshal(jsonBytes, operatorSpec); err != nil {
		return fmt.Errorf("unable to deserialize operator configuration: %w", err)
	}

	desiredSpec := &climanagerapplyconfiguration.CliManagerSpecApplyConfiguration{
		OperatorSpec: *operatorSpec,
	}
	desired := climanagerapplyconfiguration.CliManager(OperatorConfigName, OperatorNamespace)
	desired.WithSpec(desiredSpec)

	instance, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(c.Ctx, OperatorConfigName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
	// do nothing and proceed with the apply
	case err != nil:
		return fmt.Errorf("unable to get operator configuration: %w", err)
	default:
		original, err := climanagerapplyconfiguration.ExtractCLIManager(instance, fieldManager)
		if err != nil {
			return fmt.Errorf("unable to extract operator configuration: %w", err)
		}
		if equality.Semantic.DeepEqual(original, desired) {
			return nil
		}
	}

	_, err = c.OperatorClient.CliManagers(OperatorNamespace).Apply(ctx, desired, v1.ApplyOptions{
		Force:        true,
		FieldManager: fieldManager,
	})
	if err != nil {
		return fmt.Errorf("unable to Apply for operator using fieldManager %q: %w", fieldManager, err)
	}

	return nil
}

func (c *CLIManagerClient) ApplyOperatorStatus(ctx context.Context, fieldManager string, desiredConfiguration *applyconfiguration.OperatorStatusApplyConfiguration) error {
	if desiredConfiguration == nil {
		return fmt.Errorf("applyConfiguration must have a value")
	}

	jsonBytes, err := json.Marshal(desiredConfiguration)
	if err != nil {
		return fmt.Errorf("unable to serialize operator configuration: %w", err)
	}
	operatorStatus := &operatorv1.OperatorStatus{}
	if err := json.Unmarshal(jsonBytes, operatorStatus); err != nil {
		return fmt.Errorf("unable to deserialize operator configuration: %w", err)
	}

	desiredStatus := &climanagerapplyconfiguration.CliManagerStatusApplyConfiguration{
		OperatorStatus: *operatorStatus,
	}
	desired := climanagerapplyconfiguration.CliManager(OperatorConfigName, OperatorNamespace)
	desired.WithStatus(desiredStatus)

	instance, err := c.OperatorClient.CliManagers(OperatorNamespace).Get(c.Ctx, OperatorConfigName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		// do nothing and proceed with the apply
	case err != nil:
		return fmt.Errorf("unable to get operator configuration: %w", err)
	default:
		original, err := climanagerapplyconfiguration.ExtractCLIManagerStatus(instance, fieldManager)
		if err != nil {
			return fmt.Errorf("unable to extract operator configuration: %w", err)
		}
		if equality.Semantic.DeepEqual(original, desired) {
			return nil
		}
	}

	_, err = c.OperatorClient.CliManagers(OperatorNamespace).ApplyStatus(ctx, desired, v1.ApplyOptions{
		Force:        true,
		FieldManager: fieldManager,
	})
	if err != nil {
		return fmt.Errorf("unable to ApplyStatus for operator using fieldManager %q: %w", fieldManager, err)
	}

	return nil
}

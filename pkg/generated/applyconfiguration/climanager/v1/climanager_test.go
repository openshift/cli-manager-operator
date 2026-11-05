package v1

import (
	"testing"

	operatorv1 "github.com/openshift/api/operator/v1"

	climanagerv1 "github.com/openshift/cli-manager-operator/pkg/apis/climanager/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractCliManager(t *testing.T) {
	instance := &climanagerv1.CliManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster",
			Namespace: "openshift-cli-manager-operator",
			Labels:    map[string]string{"name": "openshift-cli-manager-operator"},
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:    "cli-manager-operator",
					Operation:  metav1.ManagedFieldsOperationApply,
					APIVersion: "operator.openshift.io/v1",
					FieldsType: "FieldsV1",
					FieldsV1: &metav1.FieldsV1{Raw: []byte(
						`{"f:metadata":{"f:labels":{"f:name":{}}},"f:spec":{"f:logLevel":{},"f:managementState":{}}}`)},
				},
			},
		},
		Spec: climanagerv1.CliManagerSpec{
			OperatorSpec: operatorv1.OperatorSpec{
				ManagementState:  operatorv1.Managed,
				LogLevel:         operatorv1.Debug,
				OperatorLogLevel: operatorv1.TraceAll,
			},
		},
	}

	extracted, err := ExtractCliManager(instance, "cli-manager-operator")
	if err != nil {
		t.Fatalf("unexpected error extracting applied configuration: %v", err)
	}
	if extracted.Name == nil || *extracted.Name != "cluster" {
		t.Errorf("expected name to be set to cluster, got %v", extracted.Name)
	}
	if extracted.Kind == nil || *extracted.Kind != "CliManager" {
		t.Errorf("expected kind to be set to CliManager, got %v", extracted.Kind)
	}
	if extracted.APIVersion == nil || *extracted.APIVersion != "operator.openshift.io/v1" {
		t.Errorf("expected apiVersion to be set to operator.openshift.io/v1, got %v", extracted.APIVersion)
	}
	if extracted.Spec == nil {
		t.Fatal("expected spec to be extracted")
	}
	if extracted.Spec.LogLevel == nil || *extracted.Spec.LogLevel != operatorv1.Debug {
		t.Errorf("expected logLevel to be extracted as Debug, got %v", extracted.Spec.LogLevel)
	}
	if extracted.Spec.ManagementState == nil || *extracted.Spec.ManagementState != operatorv1.Managed {
		t.Errorf("expected managementState to be extracted as Managed, got %v", extracted.Spec.ManagementState)
	}
	if extracted.Spec.OperatorLogLevel != nil {
		t.Errorf("expected operatorLogLevel to not be extracted since it is not owned by the field manager, got %v", *extracted.Spec.OperatorLogLevel)
	}
	if extracted.Labels["name"] != "openshift-cli-manager-operator" {
		t.Errorf("expected metadata.labels.name to be extracted, got %v", extracted.Labels)
	}
}

func TestExtractCliManagerStatus(t *testing.T) {
	instance := &climanagerv1.CliManager{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster",
			ManagedFields: []metav1.ManagedFieldsEntry{
				{
					Manager:     "cli-manager-operator",
					Operation:   metav1.ManagedFieldsOperationApply,
					APIVersion:  "operator.openshift.io/v1",
					Subresource: "status",
					FieldsType:  "FieldsV1",
					FieldsV1: &metav1.FieldsV1{Raw: []byte(
						`{"f:status":{"f:observedGeneration":{}}}`)},
				},
			},
		},
		Status: climanagerv1.CliManagerStatus{
			OperatorStatus: operatorv1.OperatorStatus{
				ObservedGeneration: 3,
				Conditions: []operatorv1.OperatorCondition{
					{Type: operatorv1.OperatorStatusTypeAvailable, Status: operatorv1.ConditionTrue},
				},
			},
		},
	}

	extracted, err := ExtractCliManagerStatus(instance, "cli-manager-operator")
	if err != nil {
		t.Fatalf("unexpected error extracting applied status configuration: %v", err)
	}
	if extracted.Status == nil {
		t.Fatal("expected status to be extracted")
	}
	if extracted.Status.ObservedGeneration == nil || *extracted.Status.ObservedGeneration != 3 {
		t.Errorf("expected observedGeneration to be extracted as 3, got %v", extracted.Status.ObservedGeneration)
	}
	if len(extracted.Status.Conditions) != 0 {
		t.Errorf("expected conditions to not be extracted since they are not owned by the field manager, got %v", extracted.Status.Conditions)
	}
}

func TestExtractCliManagerNoManagedFields(t *testing.T) {
	instance := &climanagerv1.CliManager{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec: climanagerv1.CliManagerSpec{
			OperatorSpec: operatorv1.OperatorSpec{ManagementState: operatorv1.Managed},
		},
	}

	extracted, err := ExtractCliManager(instance, "some-other-field-manager")
	if err != nil {
		t.Fatalf("unexpected error extracting applied configuration: %v", err)
	}
	if extracted.Spec != nil {
		t.Errorf("expected spec to not be extracted when the field manager owns no fields, got %v", extracted.Spec)
	}
	if extracted.Name == nil || *extracted.Name != "cluster" {
		t.Errorf("expected name to still be set to cluster, got %v", extracted.Name)
	}
}

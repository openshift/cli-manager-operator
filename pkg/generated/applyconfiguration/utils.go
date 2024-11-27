// Code generated by applyconfiguration-gen. DO NOT EDIT.

package applyconfiguration

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	testing "k8s.io/client-go/testing"

	v1 "github.com/openshift/cli-manager-operator/pkg/apis/climanager/v1"
	climanagerv1 "github.com/openshift/cli-manager-operator/pkg/generated/applyconfiguration/climanager/v1"
	internal "github.com/openshift/cli-manager-operator/pkg/generated/applyconfiguration/internal"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=operator.openshift.io, Version=v1
	case v1.SchemeGroupVersion.WithKind("CliManager"):
		return &climanagerv1.CliManagerApplyConfiguration{}
	case v1.SchemeGroupVersion.WithKind("CliManagerSpec"):
		return &climanagerv1.CliManagerSpecApplyConfiguration{}
	case v1.SchemeGroupVersion.WithKind("CliManagerStatus"):
		return &climanagerv1.CliManagerStatusApplyConfiguration{}

	}
	return nil
}

func NewTypeConverter(scheme *runtime.Scheme) *testing.TypeConverter {
	return &testing.TypeConverter{Scheme: scheme, TypeResolver: internal.Parser()}
}
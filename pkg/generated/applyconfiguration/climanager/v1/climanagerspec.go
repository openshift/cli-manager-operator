// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/openshift/api/operator/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// CLIManagerSpecApplyConfiguration represents an declarative configuration of the CLIManagerSpec type for use
// with apply.
type CLIManagerSpecApplyConfiguration struct {
	v1.OperatorSpec `json:",inline"`
}

// CLIManagerSpecApplyConfiguration constructs an declarative configuration of the CLIManagerSpec type for use with
// apply.
func CLIManagerSpec() *CLIManagerSpecApplyConfiguration {
	return &CLIManagerSpecApplyConfiguration{}
}

// WithManagementState sets the ManagementState field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ManagementState field is set to the value of the last call.
func (b *CLIManagerSpecApplyConfiguration) WithManagementState(value v1.ManagementState) *CLIManagerSpecApplyConfiguration {
	b.ManagementState = &value
	return b
}

// WithLogLevel sets the LogLevel field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the LogLevel field is set to the value of the last call.
func (b *CLIManagerSpecApplyConfiguration) WithLogLevel(value v1.LogLevel) *CLIManagerSpecApplyConfiguration {
	b.LogLevel = &value
	return b
}

// WithOperatorLogLevel sets the OperatorLogLevel field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the OperatorLogLevel field is set to the value of the last call.
func (b *CLIManagerSpecApplyConfiguration) WithOperatorLogLevel(value v1.LogLevel) *CLIManagerSpecApplyConfiguration {
	b.OperatorLogLevel = &value
	return b
}

// WithUnsupportedConfigOverrides sets the UnsupportedConfigOverrides field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the UnsupportedConfigOverrides field is set to the value of the last call.
func (b *CLIManagerSpecApplyConfiguration) WithUnsupportedConfigOverrides(value runtime.RawExtension) *CLIManagerSpecApplyConfiguration {
	b.UnsupportedConfigOverrides = &value
	return b
}

// WithObservedConfig sets the ObservedConfig field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ObservedConfig field is set to the value of the last call.
func (b *CLIManagerSpecApplyConfiguration) WithObservedConfig(value runtime.RawExtension) *CLIManagerSpecApplyConfiguration {
	b.ObservedConfig = &value
	return b
}

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/openshift/api/config/v1"
)

// DNSOverTLSConfigApplyConfiguration represents a declarative configuration of the DNSOverTLSConfig type for use
// with apply.
type DNSOverTLSConfigApplyConfiguration struct {
	ServerName *string                    `json:"serverName,omitempty"`
	CABundle   *v1.ConfigMapNameReference `json:"caBundle,omitempty"`
}

// DNSOverTLSConfigApplyConfiguration constructs a declarative configuration of the DNSOverTLSConfig type for use with
// apply.
func DNSOverTLSConfig() *DNSOverTLSConfigApplyConfiguration {
	return &DNSOverTLSConfigApplyConfiguration{}
}

// WithServerName sets the ServerName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ServerName field is set to the value of the last call.
func (b *DNSOverTLSConfigApplyConfiguration) WithServerName(value string) *DNSOverTLSConfigApplyConfiguration {
	b.ServerName = &value
	return b
}

// WithCABundle sets the CABundle field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CABundle field is set to the value of the last call.
func (b *DNSOverTLSConfigApplyConfiguration) WithCABundle(value v1.ConfigMapNameReference) *DNSOverTLSConfigApplyConfiguration {
	b.CABundle = &value
	return b
}
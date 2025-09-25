/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MinIOConnection defines connection details to a MinIO instance
type MinIOConnection struct {
	// EndpointRef references an Endpoint resource for connection details
	EndpointRef *EndpointReference `json:"endpointRef,omitempty"`
	// URL is the MinIO server URL (alternative to EndpointRef)
	URL *string `json:"url,omitempty"`
	// SecretRef contains credentials for connecting to MinIO
	SecretRef SecretReference `json:"secretRef"`
	// TLS configuration
	TLS *TLSConfig `json:"tls,omitempty"`
}

// EndpointReference references an Endpoint resource
type EndpointReference struct {
	// Name is the name of the Endpoint resource
	Name string `json:"name"`
	// Namespace is the namespace of the Endpoint resource
	Namespace *string `json:"namespace,omitempty"`
}

// SecretReference contains the reference to a secret containing MinIO credentials
type SecretReference struct {
	// Name is the name of the secret
	Name string `json:"name"`
	// Namespace is the namespace of the secret
	Namespace *string `json:"namespace,omitempty"`
	// AccessKeyIDKey is the key in the secret containing the access key ID
	AccessKeyIDKey string `json:"accessKeyIDKey,omitempty"`
	// SecretAccessKeyKey is the key in the secret containing the secret access key
	SecretAccessKeyKey string `json:"secretAccessKeyKey,omitempty"`
}

// TLSConfig defines TLS configuration for MinIO connection
type TLSConfig struct {
	// Insecure allows connections to MinIO using TLS without certs validation
	Insecure bool `json:"insecure,omitempty"`
	// CABundle is a PEM encoded CA bundle which will be used to validate the server certificate
	CABundle []byte `json:"caBundle,omitempty"`
}

// ConditionType represents the type of condition
type ConditionType string

const (
	// ConditionReady indicates the resource is ready
	ConditionReady ConditionType = "Ready"
	// ConditionProgressing indicates the resource is being processed
	ConditionProgressing ConditionType = "Progressing"
	// ConditionDegraded indicates the resource is in a degraded state
	ConditionDegraded ConditionType = "Degraded"
	// ConditionError indicates the resource encountered an error
	ConditionError ConditionType = "Error"
)

// Condition represents the condition of a resource
type Condition struct {
	// Type is the type of the condition
	Type ConditionType `json:"type"`
	// Status is the status of the condition
	Status metav1.ConditionStatus `json:"status"`
	// LastTransitionTime is the last time the condition transitioned
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable message indicating details about the transition
	Message string `json:"message,omitempty"`
}

// SetCondition sets a condition on a list of conditions
func SetCondition(conditions *[]Condition, conditionType ConditionType, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()

	for i, condition := range *conditions {
		if condition.Type == conditionType {
			if condition.Status != status {
				condition.Status = status
				condition.LastTransitionTime = now
				condition.Reason = reason
				condition.Message = message
			} else {
				condition.Reason = reason
				condition.Message = message
			}
			(*conditions)[i] = condition
			return
		}
	}

	*conditions = append(*conditions, Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// GetCondition returns a condition by type
func GetCondition(conditions []Condition, conditionType ConditionType) *Condition {
	for i, condition := range conditions {
		if condition.Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

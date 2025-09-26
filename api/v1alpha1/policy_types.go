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

const (
	// PolicyFinalizer is the finalizer for Policy resources
	PolicyFinalizer = "policy.mc-controller.mxcd.de/finalizer"
)

// PolicySpec defines the desired state of Policy
type PolicySpec struct {
	// Connection defines connection details to MinIO
	Connection MinIOConnection `json:"connection"`

	// PolicyName is the name of the policy in MinIO
	PolicyName string `json:"policyName"`

	// Policy is the IAM policy document in JSON format (base64 encoded when stored)
	Policy []byte `json:"policy"`

	// Description is the policy description
	Description *string `json:"description,omitempty"`

	// Tags are policy tags
	Tags map[string]string `json:"tags,omitempty"`
}

// PolicyStatus defines the observed state of Policy
type PolicyStatus struct {
	// Conditions represent the latest available observations of the policy's state
	Conditions []Condition `json:"conditions,omitempty"`

	// Ready indicates if the policy is ready
	Ready bool `json:"ready"`

	// PolicyName is the actual policy name in MinIO
	PolicyName string `json:"policyName,omitempty"`

	// PolicyHash is the hash of the policy document for comparison
	PolicyHash string `json:"policyHash,omitempty"`

	// CreationDate is when the policy was created
	CreationDate *metav1.Time `json:"creationDate,omitempty"`

	// LastSyncTime is the last time the resource was synchronized
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=miniopolicy
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Policy Name",type="string",JSONPath=".status.policyName"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Policy is the Schema for the policies API
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec,omitempty"`
	Status PolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}

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
	// PolicyAttachmentFinalizer is the finalizer for PolicyAttachment resources
	PolicyAttachmentFinalizer = "policyattachment.mc-controller.mxcd.de/finalizer"
)

// PolicyAttachmentSpec defines the desired state of PolicyAttachment
type PolicyAttachmentSpec struct {
	// Connection defines connection details to MinIO
	Connection MinIOConnection `json:"connection"`

	// PolicyName is the name of the policy to attach
	PolicyName string `json:"policyName"`

	// Target defines what the policy should be attached to
	Target PolicyAttachmentTarget `json:"target"`
}

// PolicyAttachmentTarget defines the target for policy attachment
type PolicyAttachmentTarget struct {
	// User is the username to attach the policy to
	User *string `json:"user,omitempty"`

	// Group is the group name to attach the policy to
	Group *string `json:"group,omitempty"`

	// ServiceAccount is the service account to attach the policy to
	ServiceAccount *string `json:"serviceAccount,omitempty"`
}

// PolicyAttachmentStatus defines the observed state of PolicyAttachment
type PolicyAttachmentStatus struct {
	// Conditions represent the latest available observations of the policy attachment's state
	Conditions []Condition `json:"conditions,omitempty"`

	// Ready indicates if the policy attachment is ready
	Ready bool `json:"ready"`

	// PolicyName is the actual policy name in MinIO
	PolicyName string `json:"policyName,omitempty"`

	// Target shows what the policy is attached to
	Target string `json:"target,omitempty"`

	// AttachedAt is when the policy was attached
	AttachedAt *metav1.Time `json:"attachedAt,omitempty"`

	// LastSyncTime is the last time the resource was synchronized
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=policyattach
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Policy",type="string",JSONPath=".status.policyName"
//+kubebuilder:printcolumn:name="Target",type="string",JSONPath=".status.target"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PolicyAttachment is the Schema for the policyattachments API
type PolicyAttachment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicyAttachmentSpec   `json:"spec,omitempty"`
	Status PolicyAttachmentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolicyAttachmentList contains a list of PolicyAttachment
type PolicyAttachmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PolicyAttachment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PolicyAttachment{}, &PolicyAttachmentList{})
}

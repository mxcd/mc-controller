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
	// LifecyclePolicyFinalizer is the finalizer for LifecyclePolicy resources
	LifecyclePolicyFinalizer = "lifecyclepolicy.mc-controller.mxcd.de/finalizer"
)

// LifecyclePolicySpec defines the desired state of LifecyclePolicy
type LifecyclePolicySpec struct {
	// Connection defines connection details to MinIO
	Connection MinIOConnection `json:"connection"`

	// BucketName is the name of the bucket to apply the lifecycle policy to
	BucketName string `json:"bucketName"`

	// Rules define the lifecycle rules
	Rules []LifecycleRule `json:"rules"`
}

// LifecycleRule defines a single lifecycle rule
type LifecycleRule struct {
	// ID is the unique identifier for the rule
	ID string `json:"id"`

	// Status indicates whether the rule is enabled or disabled
	Status LifecycleRuleStatus `json:"status"`

	// Filter defines the filter for objects to apply the rule to
	Filter *LifecycleFilter `json:"filter,omitempty"`

	// Expiration defines when objects expire
	Expiration *LifecycleExpiration `json:"expiration,omitempty"`

	// NoncurrentVersionExpiration defines when non-current versions expire
	NoncurrentVersionExpiration *NoncurrentVersionExpiration `json:"noncurrentVersionExpiration,omitempty"`

	// AbortIncompleteMultipartUpload defines when to abort incomplete multipart uploads
	AbortIncompleteMultipartUpload *AbortIncompleteMultipartUpload `json:"abortIncompleteMultipartUpload,omitempty"`

	// Transitions define storage class transitions
	Transitions []LifecycleTransition `json:"transitions,omitempty"`

	// NoncurrentVersionTransitions define transitions for non-current versions
	NoncurrentVersionTransitions []NoncurrentVersionTransition `json:"noncurrentVersionTransitions,omitempty"`
}

// LifecycleRuleStatus defines the status of a lifecycle rule
type LifecycleRuleStatus string

const (
	// LifecycleRuleStatusEnabled indicates the rule is enabled
	LifecycleRuleStatusEnabled LifecycleRuleStatus = "Enabled"
	// LifecycleRuleStatusDisabled indicates the rule is disabled
	LifecycleRuleStatusDisabled LifecycleRuleStatus = "Disabled"
)

// LifecycleFilter defines the filter for lifecycle rules
type LifecycleFilter struct {
	// Prefix is the object key prefix
	Prefix *string `json:"prefix,omitempty"`

	// Tags is a list of tags to match
	Tags map[string]string `json:"tags,omitempty"`

	// And allows combining multiple filters
	And *LifecycleFilterAnd `json:"and,omitempty"`
}

// LifecycleFilterAnd defines AND conditions for lifecycle filters
type LifecycleFilterAnd struct {
	// Prefix is the object key prefix
	Prefix *string `json:"prefix,omitempty"`

	// Tags is a list of tags to match
	Tags map[string]string `json:"tags,omitempty"`
}

// LifecycleExpiration defines when objects expire
type LifecycleExpiration struct {
	// Days is the number of days after creation when objects expire
	Days *int `json:"days,omitempty"`

	// Date is the expiration date
	Date *metav1.Time `json:"date,omitempty"`

	// ExpiredObjectDeleteMarker indicates whether to remove delete markers
	ExpiredObjectDeleteMarker *bool `json:"expiredObjectDeleteMarker,omitempty"`
}

// NoncurrentVersionExpiration defines when non-current versions expire
type NoncurrentVersionExpiration struct {
	// NoncurrentDays is the number of days after becoming non-current when versions expire
	NoncurrentDays int `json:"noncurrentDays"`
}

// AbortIncompleteMultipartUpload defines when to abort incomplete multipart uploads
type AbortIncompleteMultipartUpload struct {
	// DaysAfterInitiation is the number of days after initiation
	DaysAfterInitiation int `json:"daysAfterInitiation"`
}

// LifecycleTransition defines storage class transition
type LifecycleTransition struct {
	// Days is the number of days after creation when objects transition
	Days *int `json:"days,omitempty"`

	// Date is the transition date
	Date *metav1.Time `json:"date,omitempty"`

	// StorageClass is the storage class to transition to
	StorageClass string `json:"storageClass"`
}

// NoncurrentVersionTransition defines storage class transition for non-current versions
type NoncurrentVersionTransition struct {
	// NoncurrentDays is the number of days after becoming non-current when versions transition
	NoncurrentDays int `json:"noncurrentDays"`

	// StorageClass is the storage class to transition to
	StorageClass string `json:"storageClass"`
}

// LifecyclePolicyStatus defines the observed state of LifecyclePolicy
type LifecyclePolicyStatus struct {
	// Conditions represent the latest available observations of the lifecycle policy's state
	Conditions []Condition `json:"conditions,omitempty"`

	// Ready indicates if the lifecycle policy is ready
	Ready bool `json:"ready"`

	// BucketName is the actual bucket name in MinIO
	BucketName string `json:"bucketName,omitempty"`

	// PolicyHash is the hash of the policy for comparison
	PolicyHash string `json:"policyHash,omitempty"`

	// AppliedAt is when the policy was applied
	AppliedAt *metav1.Time `json:"appliedAt,omitempty"`

	// LastSyncTime is the last time the resource was synchronized
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=lifecycle
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Bucket",type="string",JSONPath=".status.bucketName"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LifecyclePolicy is the Schema for the lifecyclepolicies API
type LifecyclePolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LifecyclePolicySpec   `json:"spec,omitempty"`
	Status LifecyclePolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LifecyclePolicyList contains a list of LifecyclePolicy
type LifecyclePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LifecyclePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LifecyclePolicy{}, &LifecyclePolicyList{})
}

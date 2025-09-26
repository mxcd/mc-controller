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
	// BucketFinalizer is the finalizer for Bucket resources
	BucketFinalizer = "bucket.mc-controller.mxcd.de/finalizer"
)

// BucketSpec defines the desired state of Bucket
type BucketSpec struct {
	// Connection defines connection details to MinIO
	Connection MinIOConnection `json:"connection"`

	// BucketName is the name of the bucket to create in MinIO
	BucketName string `json:"bucketName"`

	// Region is the bucket region (optional)
	Region *string `json:"region,omitempty"`

	// ObjectLocking enables object locking on the bucket
	ObjectLocking bool `json:"objectLocking,omitempty"`

	// Versioning enables versioning on the bucket
	Versioning bool `json:"versioning,omitempty"`

	// Retention defines the default retention settings
	Retention *BucketRetention `json:"retention,omitempty"`

	// Notification defines event notification configuration
	Notification *BucketNotification `json:"notification,omitempty"`

	// Tags are bucket tags
	Tags map[string]string `json:"tags,omitempty"`

	// Quota defines storage quota for the bucket
	Quota *BucketQuota `json:"quota,omitempty"`
}

// BucketRetention defines bucket retention settings
type BucketRetention struct {
	// Mode is the retention mode (GOVERNANCE or COMPLIANCE)
	Mode string `json:"mode"`
	// RetainUntilDate is the retention date
	RetainUntilDate *metav1.Time `json:"retainUntilDate,omitempty"`
	// Years is the retention period in years
	Years *int `json:"years,omitempty"`
	// Days is the retention period in days
	Days *int `json:"days,omitempty"`
}

// BucketNotification defines bucket notification configuration
type BucketNotification struct {
	// Events is a list of events to notify on
	Events []string `json:"events"`
	// FilterPrefix is the object key name prefix
	FilterPrefix *string `json:"filterPrefix,omitempty"`
	// FilterSuffix is the object key name suffix
	FilterSuffix *string `json:"filterSuffix,omitempty"`
	// Topic is the notification target topic ARN
	Topic *string `json:"topic,omitempty"`
	// Queue is the notification target queue ARN
	Queue *string `json:"queue,omitempty"`
	// LambdaFunction is the notification target lambda function ARN
	LambdaFunction *string `json:"lambdaFunction,omitempty"`
}

// BucketQuota defines bucket storage quota
type BucketQuota struct {
	// Hard is the hard quota limit in bytes
	Hard *int64 `json:"hard,omitempty"`
}

// BucketStatus defines the observed state of Bucket
type BucketStatus struct {
	// Conditions represent the latest available observations of the bucket's state
	Conditions []Condition `json:"conditions,omitempty"`

	// Ready indicates if the bucket is ready
	Ready bool `json:"ready"`

	// BucketName is the actual bucket name in MinIO
	BucketName string `json:"bucketName,omitempty"`

	// Region is the bucket region
	Region string `json:"region,omitempty"`

	// CreationDate is when the bucket was created
	CreationDate *metav1.Time `json:"creationDate,omitempty"`

	// LastSyncTime is the last time the resource was synchronized
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=bucket
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Bucket Name",type="string",JSONPath=".status.bucketName"
//+kubebuilder:printcolumn:name="Region",type="string",JSONPath=".status.region"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Bucket is the Schema for the buckets API
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BucketList contains a list of Bucket
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}

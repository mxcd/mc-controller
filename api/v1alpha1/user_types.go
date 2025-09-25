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
	// UserFinalizer is the finalizer for User resources
	UserFinalizer = "user.minio.mxcd.dev/finalizer"
)

// UserSpec defines the desired state of User
type UserSpec struct {
	// Connection defines connection details to MinIO
	Connection MinIOConnection `json:"connection"`
	
	// Username is the MinIO username
	Username string `json:"username"`
	
	// SecretRef references a secret containing the user's password
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	
	// Password is the user's password (use SecretRef instead for security)
	Password *string `json:"password,omitempty"`
	
	// Status is the user status (enabled/disabled)
	Status UserStatusType `json:"status,omitempty"`
	
	// Groups is a list of groups the user belongs to
	Groups []string `json:"groups,omitempty"`
	
	// Policies is a list of policies attached to the user
	Policies []string `json:"policies,omitempty"`
	
	// Tags are user tags
	Tags map[string]string `json:"tags,omitempty"`
}

// UserStatusType defines the status of a user
type UserStatusType string

const (
	// UserStatusEnabled indicates the user is enabled
	UserStatusEnabled UserStatusType = "enabled"
	// UserStatusDisabled indicates the user is disabled
	UserStatusDisabled UserStatusType = "disabled"
)

// UserStatus defines the observed state of User
type UserStatus struct {
	// Conditions represent the latest available observations of the user's state
	Conditions []Condition `json:"conditions,omitempty"`
	
	// Ready indicates if the user is ready
	Ready bool `json:"ready"`
	
	// Username is the actual username in MinIO
	Username string `json:"username,omitempty"`
	
	// Status is the current user status
	Status UserStatusType `json:"status,omitempty"`
	
	// Groups is the list of groups the user belongs to
	Groups []string `json:"groups,omitempty"`
	
	// Policies is the list of policies attached to the user
	Policies []string `json:"policies,omitempty"`
	
	// CreationDate is when the user was created
	CreationDate *metav1.Time `json:"creationDate,omitempty"`
	
	// LastSyncTime is the last time the resource was synchronized
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	
	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=miniouser
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="Username",type="string",JSONPath=".status.username"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// User is the Schema for the users API
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec,omitempty"`
	Status UserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// UserList contains a list of User
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

func init() {
	SchemeBuilder.Register(&User{}, &UserList{})
}

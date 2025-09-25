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
	// EndpointFinalizer is the finalizer for Endpoint resources
	EndpointFinalizer = "endpoint.mc-controller.mxcd.de/finalizer"
)

// EndpointSpec defines the desired state of Endpoint
type EndpointSpec struct {
	// URL is the MinIO server URL
	URL string `json:"url"`

	// SecretRef contains credentials for connecting to MinIO
	SecretRef SecretReference `json:"secretRef"`

	// TLS configuration
	TLS *TLSConfig `json:"tls,omitempty"`

	// HealthCheck defines health check settings
	HealthCheck *EndpointHealthCheck `json:"healthCheck,omitempty"`

	// Region is the default region for this endpoint
	Region *string `json:"region,omitempty"`

	// PathStyle forces the use of path-style addressing
	PathStyle bool `json:"pathStyle,omitempty"`

	// Tags are endpoint tags
	Tags map[string]string `json:"tags,omitempty"`
}

// EndpointHealthCheck defines health check configuration
type EndpointHealthCheck struct {
	// Enabled indicates whether health checks are enabled
	Enabled bool `json:"enabled"`

	// IntervalSeconds is the interval between health checks in seconds
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`

	// TimeoutSeconds is the timeout for health checks in seconds
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// FailureThreshold is the number of consecutive failures before marking unhealthy
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`

	// SuccessThreshold is the number of consecutive successes before marking healthy
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
}

// EndpointStatus defines the observed state of Endpoint
type EndpointStatus struct {
	// Conditions represent the latest available observations of the endpoint's state
	Conditions []Condition `json:"conditions,omitempty"`

	// Ready indicates if the endpoint is ready
	Ready bool `json:"ready"`

	// URL is the actual endpoint URL
	URL string `json:"url,omitempty"`

	// Healthy indicates if the endpoint is healthy
	Healthy bool `json:"healthy"`

	// LastHealthCheck is the timestamp of the last health check
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// Version is the MinIO server version
	Version string `json:"version,omitempty"`

	// Region is the endpoint region
	Region string `json:"region,omitempty"`

	// ConnectedAt is when the connection was established
	ConnectedAt *metav1.Time `json:"connectedAt,omitempty"`

	// LastSyncTime is the last time the resource was synchronized
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=minioendpoint
//+kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
//+kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
//+kubebuilder:printcolumn:name="Healthy",type="boolean",JSONPath=".status.healthy"
//+kubebuilder:printcolumn:name="Version",type="string",JSONPath=".status.version"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Endpoint is the Schema for the endpoints API
type Endpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EndpointSpec   `json:"spec,omitempty"`
	Status EndpointStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EndpointList contains a list of Endpoint
type EndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Endpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Endpoint{}, &EndpointList{})
}

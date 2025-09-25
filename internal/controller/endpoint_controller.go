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

package controller

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	miniov1alpha1 "github.com/mxcd/mc-controller/api/v1alpha1"
	minioclient "github.com/mxcd/mc-controller/internal/minio"
)

// EndpointReconciler reconciles a Endpoint object
type EndpointReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=endpoints/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=endpoints/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *EndpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Endpoint instance
	endpoint := &miniov1alpha1.Endpoint{}
	err := r.Get(ctx, req.NamespacedName, endpoint)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Endpoint resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Endpoint")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if endpoint.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, endpoint)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(endpoint, miniov1alpha1.EndpointFinalizer) {
		controllerutil.AddFinalizer(endpoint, miniov1alpha1.EndpointFinalizer)
		return ctrl.Result{}, r.Update(ctx, endpoint)
	}

	// Update status to indicate reconciliation is in progress
	miniov1alpha1.SetCondition(&endpoint.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling endpoint")
	endpoint.Status.ObservedGeneration = endpoint.Generation
	if err := r.Status().Update(ctx, endpoint); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Create a temporary connection config for health checking
	conn := miniov1alpha1.MinIOConnection{
		URL:       &endpoint.Spec.URL,
		SecretRef: endpoint.Spec.SecretRef,
		TLS:       endpoint.Spec.TLS,
	}

	// Create MinIO client for health checking
	minioClient, err := minioclient.NewClient(ctx, r.Client, conn, endpoint.Namespace)
	if err != nil {
		logger.Error(err, "Failed to create MinIO client")
		miniov1alpha1.SetCondition(&endpoint.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ClientError", fmt.Sprintf("Failed to create MinIO client: %v", err))
		endpoint.Status.Ready = false
		endpoint.Status.Healthy = false
		r.Status().Update(ctx, endpoint)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile the endpoint
	result, err := r.reconcileEndpoint(ctx, endpoint, minioClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile endpoint")
		miniov1alpha1.SetCondition(&endpoint.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ReconcileError", fmt.Sprintf("Failed to reconcile endpoint: %v", err))
		endpoint.Status.Ready = false
		endpoint.Status.Healthy = false
		endpoint.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		r.Status().Update(ctx, endpoint)
		return result, err
	}

	// Update status to ready
	miniov1alpha1.SetCondition(&endpoint.Status.Conditions, miniov1alpha1.ConditionReady, metav1.ConditionTrue, "Ready", "Endpoint is ready")
	miniov1alpha1.SetCondition(&endpoint.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionFalse, "Ready", "Endpoint reconciliation completed")
	endpoint.Status.Ready = true
	endpoint.Status.URL = endpoint.Spec.URL
	endpoint.Status.LastSyncTime = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, endpoint); err != nil {
		logger.Error(err, "Failed to update status to ready")
		return ctrl.Result{}, err
	}

	return result, nil
}

// handleDeletion handles the deletion of an Endpoint resource
func (r *EndpointReconciler) handleDeletion(ctx context.Context, endpoint *miniov1alpha1.Endpoint) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(endpoint, miniov1alpha1.EndpointFinalizer) {
		// For endpoints, we don't need to do any cleanup in MinIO
		// Just remove the finalizer
		logger.Info("Endpoint being deleted", "url", endpoint.Spec.URL)
		controllerutil.RemoveFinalizer(endpoint, miniov1alpha1.EndpointFinalizer)
		return ctrl.Result{}, r.Update(ctx, endpoint)
	}

	return ctrl.Result{}, nil
}

// reconcileEndpoint reconciles the endpoint state
func (r *EndpointReconciler) reconcileEndpoint(ctx context.Context, endpoint *miniov1alpha1.Endpoint, minioClient *minioclient.Client) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Perform health check
	now := metav1.Time{Time: time.Now()}
	endpoint.Status.LastHealthCheck = &now

	err := minioClient.HealthCheck(ctx)
	if err != nil {
		logger.Error(err, "Health check failed")
		endpoint.Status.Healthy = false
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	endpoint.Status.Healthy = true
	if endpoint.Status.ConnectedAt == nil {
		endpoint.Status.ConnectedAt = &now
	}

	// Get server info to populate version and region
	serverInfo, err := minioClient.GetServerInfo(ctx)
	if err != nil {
		logger.Error(err, "Failed to get server info")
		// Don't fail reconciliation for this
	} else {
		endpoint.Status.Version = serverInfo.MinioVersion
		if endpoint.Spec.Region != nil {
			endpoint.Status.Region = *endpoint.Spec.Region
		}
	}

	// Calculate next health check interval
	interval := time.Minute * 5 // Default 5 minutes
	if endpoint.Spec.HealthCheck != nil && endpoint.Spec.HealthCheck.Enabled {
		if endpoint.Spec.HealthCheck.IntervalSeconds != nil {
			interval = time.Duration(*endpoint.Spec.HealthCheck.IntervalSeconds) * time.Second
		}
	}

	logger.Info("Endpoint health check successful", "url", endpoint.Spec.URL, "version", endpoint.Status.Version)
	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EndpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.Endpoint{}).
		Complete(r)
}

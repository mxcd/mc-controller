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

// AliasReconciler reconciles a Alias object
type AliasReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=aliases,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=aliases/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=aliases/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AliasReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Alias instance
	alias := &miniov1alpha1.Alias{}
	err := r.Get(ctx, req.NamespacedName, alias)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Alias resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Alias")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if alias.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, alias)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(alias, miniov1alpha1.AliasFinalizer) {
		controllerutil.AddFinalizer(alias, miniov1alpha1.AliasFinalizer)
		return ctrl.Result{}, r.Update(ctx, alias)
	}

	// Update status to indicate reconciliation is in progress
	miniov1alpha1.SetCondition(&alias.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling alias")
	alias.Status.ObservedGeneration = alias.Generation
	if err := r.Status().Update(ctx, alias); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Create a temporary connection config for health checking
	conn := miniov1alpha1.MinIOConnection{
		URL:       &alias.Spec.URL,
		SecretRef: &alias.Spec.SecretRef,
		TLS:       alias.Spec.TLS,
	}

	// Create MinIO client for health checking
	minioClient, err := minioclient.NewClient(ctx, r.Client, conn, alias.Namespace)
	if err != nil {
		logger.Error(err, "Failed to create MinIO client")
		miniov1alpha1.SetCondition(&alias.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ClientError", fmt.Sprintf("Failed to create MinIO client: %v", err))
		alias.Status.Ready = false
		alias.Status.Healthy = false
		r.Status().Update(ctx, alias)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile the alias
	result, err := r.reconcileAlias(ctx, alias, minioClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile alias")
		miniov1alpha1.SetCondition(&alias.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ReconcileError", fmt.Sprintf("Failed to reconcile alias: %v", err))
		alias.Status.Ready = false
		alias.Status.Healthy = false
		alias.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		r.Status().Update(ctx, alias)
		return result, err
	}

	// Update status to ready
	miniov1alpha1.SetCondition(&alias.Status.Conditions, miniov1alpha1.ConditionReady, metav1.ConditionTrue, "Ready", "Alias is ready")
	miniov1alpha1.SetCondition(&alias.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionFalse, "Ready", "Alias reconciliation completed")
	alias.Status.Ready = true
	alias.Status.URL = alias.Spec.URL
	alias.Status.LastSyncTime = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, alias); err != nil {
		logger.Error(err, "Failed to update status to ready")
		return ctrl.Result{}, err
	}

	return result, nil
}

// handleDeletion handles the deletion of an Alias resource
func (r *AliasReconciler) handleDeletion(ctx context.Context, alias *miniov1alpha1.Alias) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(alias, miniov1alpha1.AliasFinalizer) {
		// For aliases, we don't need to do any cleanup in MinIO
		// Just remove the finalizer
		logger.Info("Alias being deleted", "url", alias.Spec.URL)
		controllerutil.RemoveFinalizer(alias, miniov1alpha1.AliasFinalizer)
		return ctrl.Result{}, r.Update(ctx, alias)
	}

	return ctrl.Result{}, nil
}

// reconcileAlias reconciles the alias state
func (r *AliasReconciler) reconcileAlias(ctx context.Context, alias *miniov1alpha1.Alias, minioClient *minioclient.Client) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Perform health check
	now := metav1.Time{Time: time.Now()}
	alias.Status.LastHealthCheck = &now

	err := minioClient.HealthCheck(ctx)
	if err != nil {
		logger.Error(err, "Health check failed")
		alias.Status.Healthy = false
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	alias.Status.Healthy = true
	if alias.Status.ConnectedAt == nil {
		alias.Status.ConnectedAt = &now
	}

	// Get server info to populate version and region
	serverInfo, err := minioClient.GetServerInfo(ctx)
	if err != nil {
		logger.Error(err, "Failed to get server info")
		// Don't fail reconciliation for this
	} else {
		alias.Status.Version = serverInfo.MinioVersion
		if alias.Spec.Region != nil {
			alias.Status.Region = *alias.Spec.Region
		}
	}

	// Calculate next health check interval
	interval := time.Minute * 5 // Default 5 minutes
	if alias.Spec.HealthCheck != nil && alias.Spec.HealthCheck.Enabled {
		if alias.Spec.HealthCheck.IntervalSeconds != nil {
			interval = time.Duration(*alias.Spec.HealthCheck.IntervalSeconds) * time.Second
		}
	}

	logger.Info("Alias health check successful", "url", alias.Spec.URL, "version", alias.Status.Version)
	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AliasReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.Alias{}).
		Complete(r)
}

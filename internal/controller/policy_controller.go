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
	"crypto/sha256"
	"encoding/hex"
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

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=policies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=policies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=policies/finalizers,verbs=update
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=aliases;endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Policy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Policy instance
	policy := &miniov1alpha1.Policy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if apierrors.IsNotFound(err) {
			// Object deleted
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if policy.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, policy)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(policy, miniov1alpha1.PolicyFinalizer) {
		controllerutil.AddFinalizer(policy, miniov1alpha1.PolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Mark progressing
	miniov1alpha1.SetCondition(&policy.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling policy")
	policy.Status.ObservedGeneration = policy.Generation
	if err := r.Status().Update(ctx, policy); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// Build MinIO client
	minioClient, err := minioclient.NewClient(ctx, r.Client, policy.Spec.Connection, policy.Namespace)
	if err != nil {
		logger.Error(err, "Failed to create MinIO client")
		miniov1alpha1.SetCondition(&policy.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ClientError", fmt.Sprintf("Failed to create MinIO client: %v", err))
		policy.Status.Ready = false
		r.Status().Update(ctx, policy)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile external policy
	result, err := r.reconcilePolicy(ctx, policy, minioClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile policy")
		miniov1alpha1.SetCondition(&policy.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ReconcileError", fmt.Sprintf("Failed to reconcile policy: %v", err))
		policy.Status.Ready = false
		policy.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		_ = r.Status().Update(ctx, policy)
		return result, err
	}

	// Mark ready
	miniov1alpha1.SetCondition(&policy.Status.Conditions, miniov1alpha1.ConditionReady, metav1.ConditionTrue, "Ready", "Policy is ready")
	miniov1alpha1.SetCondition(&policy.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionFalse, "Ready", "Policy reconciliation completed")
	policy.Status.Ready = true
	policy.Status.PolicyName = policy.Spec.PolicyName
	policy.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, policy); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	return result, nil
}

// handleDeletion handles deletion and finalizer logic for Policy
func (r *PolicyReconciler) handleDeletion(ctx context.Context, policy *miniov1alpha1.Policy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(policy, miniov1alpha1.PolicyFinalizer) {
		// Try to create client to remove external resource
		minioClient, err := minioclient.NewClient(ctx, r.Client, policy.Spec.Connection, policy.Namespace)
		if err == nil {
			// Attempt to remove canned policy (ignore not found)
			if err := minioClient.Admin.RemoveCannedPolicy(ctx, policy.Spec.PolicyName); err != nil {
				logger.Error(err, "Failed to remove canned policy, will retry", "policyName", policy.Spec.PolicyName)
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}
			logger.Info("Removed canned policy", "policyName", policy.Spec.PolicyName)
		} else {
			logger.Error(err, "Failed to create MinIO client for deletion, retrying")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		// Remove finalizer
		controllerutil.RemoveFinalizer(policy, miniov1alpha1.PolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcilePolicy ensures the MinIO canned policy matches desired spec
func (r *PolicyReconciler) reconcilePolicy(ctx context.Context, policy *miniov1alpha1.Policy, minioClient *minioclient.Client) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desiredBytes := policy.Spec.Policy
	if len(desiredBytes) == 0 {
		// If empty (should not happen because required), mark error
		return ctrl.Result{}, fmt.Errorf("policy document is empty")
	}

	sum := sha256.Sum256(desiredBytes)
	hash := hex.EncodeToString(sum[:])

	// Get existing policy (if any)
	existing, err := minioClient.Admin.InfoCannedPolicy(ctx, policy.Spec.PolicyName)
	exists := err == nil && len(existing) > 0

	// Only update if new or content changed
	if !exists || policy.Status.PolicyHash != hash {
		if err := minioClient.Admin.AddCannedPolicy(ctx, policy.Spec.PolicyName, desiredBytes); err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to add/update canned policy: %w", err)
		}
		logger.Info("Applied canned policy", "policyName", policy.Spec.PolicyName, "updated", exists)
		if !exists {
			policy.Status.CreationDate = &metav1.Time{Time: time.Now()}
		}
	}

	policy.Status.PolicyHash = hash
	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.Policy{}).
		Complete(r)
}

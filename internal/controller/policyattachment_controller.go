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

// PolicyAttachmentReconciler reconciles a PolicyAttachment object
type PolicyAttachmentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=policyattachments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=policyattachments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=policyattachments/finalizers,verbs=update
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=aliases;endpoints;users,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *PolicyAttachmentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	attachment := &miniov1alpha1.PolicyAttachment{}
	if err := r.Get(ctx, req.NamespacedName, attachment); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Deletion handling
	if attachment.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, attachment)
	}

	// Add finalizer
	if !controllerutil.ContainsFinalizer(attachment, miniov1alpha1.PolicyAttachmentFinalizer) {
		controllerutil.AddFinalizer(attachment, miniov1alpha1.PolicyAttachmentFinalizer)
		if err := r.Update(ctx, attachment); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Progressing status
	miniov1alpha1.SetCondition(&attachment.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling policy attachment")
	attachment.Status.ObservedGeneration = attachment.Generation
	if err := r.Status().Update(ctx, attachment); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	// Build MinIO client
	minioClient, err := minioclient.NewClient(ctx, r.Client, attachment.Spec.Connection, attachment.Namespace)
	if err != nil {
		logger.Error(err, "Failed to create MinIO client")
		miniov1alpha1.SetCondition(&attachment.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ClientError", fmt.Sprintf("Failed to create MinIO client: %v", err))
		attachment.Status.Ready = false
		_ = r.Status().Update(ctx, attachment)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile external state
	result, err := r.reconcileAttachment(ctx, attachment, minioClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile policy attachment")
		miniov1alpha1.SetCondition(&attachment.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ReconcileError", fmt.Sprintf("Failed to reconcile policy attachment: %v", err))
		attachment.Status.Ready = false
		attachment.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		_ = r.Status().Update(ctx, attachment)
		return result, err
	}

	miniov1alpha1.SetCondition(&attachment.Status.Conditions, miniov1alpha1.ConditionReady, metav1.ConditionTrue, "Ready", "Policy attachment is ready")
	miniov1alpha1.SetCondition(&attachment.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionFalse, "Ready", "Policy attachment reconciliation completed")
	attachment.Status.Ready = true
	attachment.Status.PolicyName = attachment.Spec.PolicyName
	attachment.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
	if attachment.Status.AttachedAt == nil {
		attachment.Status.AttachedAt = &metav1.Time{Time: time.Now()}
	}
	if err := r.Status().Update(ctx, attachment); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	return result, nil
}

func (r *PolicyAttachmentReconciler) handleDeletion(ctx context.Context, attachment *miniov1alpha1.PolicyAttachment) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(attachment, miniov1alpha1.PolicyAttachmentFinalizer) {
		minioClient, err := minioclient.NewClient(ctx, r.Client, attachment.Spec.Connection, attachment.Namespace)
		if err == nil {
			// Detach policy by setting empty policy
			target, isGroup, err2 := resolveTarget(attachment.Spec.Target)
			if err2 == nil && target != "" {
				if err3 := minioClient.Admin.SetPolicy(ctx, "", target, isGroup); err3 != nil {
					logger.Error(err3, "Failed to detach policy (will retry)", "target", target)
					return ctrl.Result{RequeueAfter: time.Minute}, nil
				}
				logger.Info("Detached policy from target", "target", target)
			}
		} else {
			logger.Error(err, "Failed to create MinIO client for deletion (retrying)")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		controllerutil.RemoveFinalizer(attachment, miniov1alpha1.PolicyAttachmentFinalizer)
		if err := r.Update(ctx, attachment); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *PolicyAttachmentReconciler) reconcileAttachment(ctx context.Context, attachment *miniov1alpha1.PolicyAttachment, minioClient *minioclient.Client) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	target, isGroup, err := resolveTarget(attachment.Spec.Target)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid target: %w", err)
	}

	// Validate target existence
	if isGroup {
		return ctrl.Result{}, fmt.Errorf("group targets not implemented")
	} else {
		_, err := minioClient.Admin.GetUserInfo(ctx, target)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("user %s not found or not ready: %w", target, err)
		}
	}

	// Attach policy
	if err := minioClient.Admin.SetPolicy(ctx, attachment.Spec.PolicyName, target, isGroup); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to set policy: %w", err)
	}

	attachment.Status.Target = target
	logger.Info("Attached policy", "policy", attachment.Spec.PolicyName, "target", target, "group", isGroup)

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

func resolveTarget(t miniov1alpha1.PolicyAttachmentTarget) (string, bool, error) {
	count := 0
	var name string
	var isGroup bool

	if t.User != nil && *t.User != "" {
		count++
		name = *t.User
		isGroup = false
	}
	if t.Group != nil && *t.Group != "" {
		count++
		name = *t.Group
		isGroup = true
	}
	if t.ServiceAccount != nil && *t.ServiceAccount != "" {
		// ServiceAccount not implemented yet
		count++
	}

	if count != 1 {
		return "", false, fmt.Errorf("exactly one of user or group (serviceAccount not supported) must be specified")
	}
	return name, isGroup, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyAttachmentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.PolicyAttachment{}).
		Complete(r)
}

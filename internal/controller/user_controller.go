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

	"github.com/minio/madmin-go/v3"
	corev1 "k8s.io/api/core/v1"
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

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=users,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=users/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=users/finalizers,verbs=update
//+kubebuilder:rbac:groups=mc-controller.mxcd.de,resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the User instance
	user := &miniov1alpha1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("User resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get User")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if user.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, user)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(user, miniov1alpha1.UserFinalizer) {
		controllerutil.AddFinalizer(user, miniov1alpha1.UserFinalizer)
		return ctrl.Result{}, r.Update(ctx, user)
	}

	// Update status to indicate reconciliation is in progress
	miniov1alpha1.SetCondition(&user.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling user")
	user.Status.ObservedGeneration = user.Generation
	if err := r.Status().Update(ctx, user); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Create MinIO client
	minioClient, err := minioclient.NewClient(ctx, r.Client, user.Spec.Connection, user.Namespace)
	if err != nil {
		logger.Error(err, "Failed to create MinIO client")
		miniov1alpha1.SetCondition(&user.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ClientError", fmt.Sprintf("Failed to create MinIO client: %v", err))
		user.Status.Ready = false
		r.Status().Update(ctx, user)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile the user
	result, err := r.reconcileUser(ctx, user, minioClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile user")
		miniov1alpha1.SetCondition(&user.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ReconcileError", fmt.Sprintf("Failed to reconcile user: %v", err))
		user.Status.Ready = false
		user.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		r.Status().Update(ctx, user)
		return result, err
	}

	// Update status to ready
	miniov1alpha1.SetCondition(&user.Status.Conditions, miniov1alpha1.ConditionReady, metav1.ConditionTrue, "Ready", "User is ready")
	miniov1alpha1.SetCondition(&user.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionFalse, "Ready", "User reconciliation completed")
	user.Status.Ready = true
	user.Status.Username = user.Spec.Username
	user.Status.LastSyncTime = &metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, user); err != nil {
		logger.Error(err, "Failed to update status to ready")
		return ctrl.Result{}, err
	}

	return result, nil
}

// handleDeletion handles the deletion of a User resource
func (r *UserReconciler) handleDeletion(ctx context.Context, user *miniov1alpha1.User) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(user, miniov1alpha1.UserFinalizer) {
		// Create MinIO client for cleanup
		minioClient, err := minioclient.NewClient(ctx, r.Client, user.Spec.Connection, user.Namespace)
		if err != nil {
			logger.Error(err, "Failed to create MinIO client for deletion")
			// If we can't connect to MinIO, we'll remove the finalizer anyway after a grace period
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		// Check if user exists and delete it
		_, err = minioClient.Admin.GetUserInfo(ctx, user.Spec.Username)
		if err == nil {
			// User exists, delete it
			err = minioClient.Admin.RemoveUser(ctx, user.Spec.Username)
			if err != nil {
				logger.Error(err, "Failed to delete user")
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}
			logger.Info("User deleted successfully", "username", user.Spec.Username)
		}

		// Remove the finalizer
		controllerutil.RemoveFinalizer(user, miniov1alpha1.UserFinalizer)
		return ctrl.Result{}, r.Update(ctx, user)
	}

	return ctrl.Result{}, nil
}

// reconcileUser reconciles the user state
func (r *UserReconciler) reconcileUser(ctx context.Context, user *miniov1alpha1.User, minioClient *minioclient.Client) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get password from secret or spec
	password, err := r.getPassword(ctx, user)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to get password: %w", err)
	}

	// Check if user exists
	_, err = minioClient.Admin.GetUserInfo(ctx, user.Spec.Username)
	userExists := err == nil

	if !userExists {
		// Create the user
		err = minioClient.Admin.AddUser(ctx, user.Spec.Username, password)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to create user: %w", err)
		}
		logger.Info("User created successfully", "username", user.Spec.Username)
		user.Status.CreationDate = &metav1.Time{Time: time.Now()}
	} else {
		// Update password if needed (we can't compare current password)
		err = minioClient.Admin.SetUser(ctx, user.Spec.Username, password, madmin.AccountEnabled)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to update user: %w", err)
		}
	}

	// Set user status
	status := madmin.AccountEnabled
	if user.Spec.Status == miniov1alpha1.UserStatusDisabled {
		status = madmin.AccountDisabled
	}

	err = minioClient.Admin.SetUserStatus(ctx, user.Spec.Username, status)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to set user status: %w", err)
	}

	// Update status fields
	user.Status.Status = user.Spec.Status
	user.Status.Groups = user.Spec.Groups
	user.Status.Policies = user.Spec.Policies

	// Set user policies
	if len(user.Spec.Policies) > 0 {
		err = minioClient.Admin.SetPolicy(ctx, user.Spec.Policies[0], user.Spec.Username, false)
		if err != nil {
			logger.Error(err, "Failed to set user policy (non-fatal)")
		}
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

// getPassword retrieves the password from secret or spec
func (r *UserReconciler) getPassword(ctx context.Context, user *miniov1alpha1.User) (string, error) {
	if user.Spec.Password != nil {
		return *user.Spec.Password, nil
	}

	if user.Spec.SecretRef == nil {
		return "", fmt.Errorf("either password or secretRef must be specified")
	}

	secretRef := user.Spec.SecretRef
	secretNamespace := user.Namespace
	if secretRef.Namespace != nil {
		secretNamespace = *secretRef.Namespace
	}

	secret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      secretRef.Name,
		Namespace: secretNamespace,
	}, secret)
	if err != nil {
		return "", fmt.Errorf("failed to get password secret: %w", err)
	}

	// Use default key "password" if not specified
	passwordKey := "password"
	if secretRef.SecretAccessKeyKey != "" {
		passwordKey = secretRef.SecretAccessKeyKey
	}

	passwordBytes, ok := secret.Data[passwordKey]
	if !ok {
		return "", fmt.Errorf("password not found in secret with key %s", passwordKey)
	}

	return string(passwordBytes), nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.User{}).
		Complete(r)
}

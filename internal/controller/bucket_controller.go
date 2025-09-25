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

	"github.com/minio/minio-go/v7"
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

// BucketReconciler reconciles a Bucket object
type BucketReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=minio.mxcd.dev,resources=buckets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minio.mxcd.dev,resources=buckets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minio.mxcd.dev,resources=buckets/finalizers,verbs=update
//+kubebuilder:rbac:groups=minio.mxcd.dev,resources=endpoints,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Bucket instance
	bucket := &miniov1alpha1.Bucket{}
	err := r.Get(ctx, req.NamespacedName, bucket)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Bucket resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Bucket")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if bucket.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, bucket)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(bucket, miniov1alpha1.BucketFinalizer) {
		controllerutil.AddFinalizer(bucket, miniov1alpha1.BucketFinalizer)
		return ctrl.Result{}, r.Update(ctx, bucket)
	}

	// Update status to indicate reconciliation is in progress
	miniov1alpha1.SetCondition(&bucket.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionTrue, "Reconciling", "Reconciling bucket")
	bucket.Status.ObservedGeneration = bucket.Generation
	if err := r.Status().Update(ctx, bucket); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	// Create MinIO client
	minioClient, err := minioclient.NewClient(ctx, r.Client, bucket.Spec.Connection, bucket.Namespace)
	if err != nil {
		logger.Error(err, "Failed to create MinIO client")
		miniov1alpha1.SetCondition(&bucket.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ClientError", fmt.Sprintf("Failed to create MinIO client: %v", err))
		bucket.Status.Ready = false
		r.Status().Update(ctx, bucket)
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// Reconcile the bucket
	result, err := r.reconcileBucket(ctx, bucket, minioClient)
	if err != nil {
		logger.Error(err, "Failed to reconcile bucket")
		miniov1alpha1.SetCondition(&bucket.Status.Conditions, miniov1alpha1.ConditionError, metav1.ConditionTrue, "ReconcileError", fmt.Sprintf("Failed to reconcile bucket: %v", err))
		bucket.Status.Ready = false
		bucket.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
		r.Status().Update(ctx, bucket)
		return result, err
	}

	// Update status to ready
	miniov1alpha1.SetCondition(&bucket.Status.Conditions, miniov1alpha1.ConditionReady, metav1.ConditionTrue, "Ready", "Bucket is ready")
	miniov1alpha1.SetCondition(&bucket.Status.Conditions, miniov1alpha1.ConditionProgressing, metav1.ConditionFalse, "Ready", "Bucket reconciliation completed")
	bucket.Status.Ready = true
	bucket.Status.BucketName = bucket.Spec.BucketName
	bucket.Status.LastSyncTime = &metav1.Time{Time: time.Now()}
	
	if err := r.Status().Update(ctx, bucket); err != nil {
		logger.Error(err, "Failed to update status to ready")
		return ctrl.Result{}, err
	}

	return result, nil
}

// handleDeletion handles the deletion of a Bucket resource
func (r *BucketReconciler) handleDeletion(ctx context.Context, bucket *miniov1alpha1.Bucket) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if controllerutil.ContainsFinalizer(bucket, miniov1alpha1.BucketFinalizer) {
		// Create MinIO client for cleanup
		minioClient, err := minioclient.NewClient(ctx, r.Client, bucket.Spec.Connection, bucket.Namespace)
		if err != nil {
			logger.Error(err, "Failed to create MinIO client for deletion")
			// If we can't connect to MinIO, we'll remove the finalizer anyway after a grace period
			// to avoid blocking deletion indefinitely
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		// Check if bucket exists and delete it
		exists, err := minioClient.S3.BucketExists(ctx, bucket.Spec.BucketName)
		if err != nil {
			logger.Error(err, "Failed to check bucket existence during deletion")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}

		if exists {
			// Remove all objects from bucket first
			err = r.emptyBucket(ctx, minioClient, bucket.Spec.BucketName)
			if err != nil {
				logger.Error(err, "Failed to empty bucket during deletion")
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}

			// Remove the bucket
			err = minioClient.S3.RemoveBucket(ctx, bucket.Spec.BucketName)
			if err != nil {
				logger.Error(err, "Failed to delete bucket")
				return ctrl.Result{RequeueAfter: time.Minute}, nil
			}
			logger.Info("Bucket deleted successfully", "bucketName", bucket.Spec.BucketName)
		}

		// Remove the finalizer
		controllerutil.RemoveFinalizer(bucket, miniov1alpha1.BucketFinalizer)
		return ctrl.Result{}, r.Update(ctx, bucket)
	}

	return ctrl.Result{}, nil
}

// emptyBucket removes all objects from a bucket
func (r *BucketReconciler) emptyBucket(ctx context.Context, minioClient *minioclient.Client, bucketName string) error {
	objectsCh := minioClient.S3.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})

	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("error listing objects: %w", object.Err)
		}

		err := minioClient.S3.RemoveObject(ctx, bucketName, object.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return fmt.Errorf("error removing object %s: %w", object.Key, err)
		}
	}

	return nil
}

// reconcileBucket reconciles the bucket state
func (r *BucketReconciler) reconcileBucket(ctx context.Context, bucket *miniov1alpha1.Bucket, minioClient *minioclient.Client) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Check if bucket exists
	exists, err := minioClient.S3.BucketExists(ctx, bucket.Spec.BucketName)
	if err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		// Create the bucket
		opts := minio.MakeBucketOptions{
			ObjectLocking: bucket.Spec.ObjectLocking,
		}
		if bucket.Spec.Region != nil {
			opts.Region = *bucket.Spec.Region
			bucket.Status.Region = *bucket.Spec.Region
		}

		err = minioClient.S3.MakeBucket(ctx, bucket.Spec.BucketName, opts)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Info("Bucket created successfully", "bucketName", bucket.Spec.BucketName)
		bucket.Status.CreationDate = &metav1.Time{Time: time.Now()}
	}

	// Configure bucket versioning if specified
	if bucket.Spec.Versioning {
		err = minioClient.S3.SetBucketVersioning(ctx, bucket.Spec.BucketName, minio.BucketVersioningConfiguration{
			Status: "Enabled",
		})
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to enable versioning: %w", err)
		}
	}

	// Set bucket tags if specified
	if len(bucket.Spec.Tags) > 0 {
		tags, err := minio.NewTags()
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to create tags: %w", err)
		}
		for key, value := range bucket.Spec.Tags {
			err = tags.Set(key, value)
			if err != nil {
				return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to set tag %s: %w", key, err)
			}
		}
		err = minioClient.S3.SetBucketTagging(ctx, bucket.Spec.BucketName, tags)
		if err != nil {
			return ctrl.Result{RequeueAfter: time.Minute}, fmt.Errorf("failed to set bucket tags: %w", err)
		}
	}

	return ctrl.Result{RequeueAfter: time.Hour}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&miniov1alpha1.Bucket{}).
		Complete(r)
}

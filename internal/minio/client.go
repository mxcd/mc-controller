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

package minio

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	miniov1alpha1 "github.com/mxcd/mc-controller/api/v1alpha1"
)

// Client wraps MinIO client and admin client
type Client struct {
	S3     *minio.Client
	Admin  *madmin.AdminClient
	config ClientConfig
}

// ClientConfig holds configuration for MinIO client
type ClientConfig struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	UseSSL          bool
	Insecure        bool
	PathStyle       bool
	Region          string
}

// NewClient creates a new MinIO client from connection configuration
func NewClient(ctx context.Context, k8sClient client.Client, conn miniov1alpha1.MinIOConnection, namespace string) (*Client, error) {
	config, err := buildClientConfig(ctx, k8sClient, conn, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to build client config: %w", err)
	}

	// Create S3 client
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: config.Insecure,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	// Force path style if requested
	if config.PathStyle {
		minioClient.SetAppInfo("mc-controller", "1.0.0")
	}

	// Create admin client
	adminClient, err := madmin.New(config.Endpoint, config.AccessKeyID, config.SecretAccessKey, config.UseSSL)
	if err != nil {
		return nil, fmt.Errorf("failed to create admin client: %w", err)
	}

	if config.Insecure {
		adminClient.SetCustomTransport(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		})
	}

	return &Client{
		S3:     minioClient,
		Admin:  adminClient,
		config: *config,
	}, nil
}

// buildClientConfig builds client configuration from connection spec
func buildClientConfig(ctx context.Context, k8sClient client.Client, conn miniov1alpha1.MinIOConnection, defaultNamespace string) (*ClientConfig, error) {
	config := &ClientConfig{
		UseSSL:    true, // Default to SSL
		PathStyle: false,
	}

	// Get endpoint URL - prioritize AliasRef over EndpointRef
	if conn.AliasRef != nil {
		alias := &miniov1alpha1.Alias{}
		aliasNamespace := defaultNamespace
		if conn.AliasRef.Namespace != nil {
			aliasNamespace = *conn.AliasRef.Namespace
		}

		err := k8sClient.Get(ctx, client.ObjectKey{
			Name:      conn.AliasRef.Name,
			Namespace: aliasNamespace,
		}, alias)
		if err != nil {
			return nil, fmt.Errorf("failed to get alias %s/%s: %w", aliasNamespace, conn.AliasRef.Name, err)
		}

		if !alias.Status.Ready {
			return nil, fmt.Errorf("alias %s/%s is not ready", aliasNamespace, conn.AliasRef.Name)
		}

		config.Endpoint = alias.Spec.URL
		if alias.Spec.PathStyle {
			config.PathStyle = true
		}
		if alias.Spec.Region != nil {
			config.Region = *alias.Spec.Region
		}

		// Use TLS config from alias if specified
		if alias.Spec.TLS != nil {
			config.Insecure = alias.Spec.TLS.Insecure
		}

		// Get credentials from alias secret
		secretNamespace := aliasNamespace
		if alias.Spec.SecretRef.Namespace != nil {
			secretNamespace = *alias.Spec.SecretRef.Namespace
		}

		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Name:      alias.Spec.SecretRef.Name,
			Namespace: secretNamespace,
		}, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get alias secret %s/%s: %w", secretNamespace, alias.Spec.SecretRef.Name, err)
		}

		// Get access key ID
		accessKeyIDKey := alias.Spec.SecretRef.AccessKeyIDKey
		if accessKeyIDKey == "" {
			accessKeyIDKey = "accessKeyID"
		}
		accessKeyIDBytes, ok := secret.Data[accessKeyIDKey]
		if !ok {
			return nil, fmt.Errorf("access key ID not found in alias secret %s/%s with key %s", secretNamespace, alias.Spec.SecretRef.Name, accessKeyIDKey)
		}
		config.AccessKeyID = string(accessKeyIDBytes)

		// Get secret access key
		secretAccessKeyKey := alias.Spec.SecretRef.SecretAccessKeyKey
		if secretAccessKeyKey == "" {
			secretAccessKeyKey = "secretAccessKey"
		}
		secretAccessKeyBytes, ok := secret.Data[secretAccessKeyKey]
		if !ok {
			return nil, fmt.Errorf("secret access key not found in alias secret %s/%s with key %s", secretNamespace, alias.Spec.SecretRef.Name, secretAccessKeyKey)
		}
		config.SecretAccessKey = string(secretAccessKeyBytes)

	} else if conn.URL != nil {
		config.Endpoint = *conn.URL
	} else if conn.EndpointRef != nil {
		endpoint := &miniov1alpha1.Endpoint{}
		endpointNamespace := defaultNamespace
		if conn.EndpointRef.Namespace != nil {
			endpointNamespace = *conn.EndpointRef.Namespace
		}

		err := k8sClient.Get(ctx, client.ObjectKey{
			Name:      conn.EndpointRef.Name,
			Namespace: endpointNamespace,
		}, endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to get endpoint %s/%s: %w", endpointNamespace, conn.EndpointRef.Name, err)
		}

		if !endpoint.Status.Ready {
			return nil, fmt.Errorf("endpoint %s/%s is not ready", endpointNamespace, conn.EndpointRef.Name)
		}

		config.Endpoint = endpoint.Spec.URL
		if endpoint.Spec.PathStyle {
			config.PathStyle = true
		}
		if endpoint.Spec.Region != nil {
			config.Region = *endpoint.Spec.Region
		}

		// Use TLS config from endpoint if specified
		if endpoint.Spec.TLS != nil {
			config.Insecure = endpoint.Spec.TLS.Insecure
		}

		// Get credentials from endpoint secret
		secretNamespace := endpointNamespace
		if endpoint.Spec.SecretRef.Namespace != nil {
			secretNamespace = *endpoint.Spec.SecretRef.Namespace
		}

		secret := &corev1.Secret{}
		err = k8sClient.Get(ctx, client.ObjectKey{
			Name:      endpoint.Spec.SecretRef.Name,
			Namespace: secretNamespace,
		}, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get endpoint secret %s/%s: %w", secretNamespace, endpoint.Spec.SecretRef.Name, err)
		}

		// Get access key ID
		accessKeyIDKey := endpoint.Spec.SecretRef.AccessKeyIDKey
		if accessKeyIDKey == "" {
			accessKeyIDKey = "accessKeyID"
		}
		accessKeyIDBytes, ok := secret.Data[accessKeyIDKey]
		if !ok {
			return nil, fmt.Errorf("access key ID not found in endpoint secret %s/%s with key %s", secretNamespace, endpoint.Spec.SecretRef.Name, accessKeyIDKey)
		}
		config.AccessKeyID = string(accessKeyIDBytes)

		// Get secret access key
		secretAccessKeyKey := endpoint.Spec.SecretRef.SecretAccessKeyKey
		if secretAccessKeyKey == "" {
			secretAccessKeyKey = "secretAccessKey"
		}
		secretAccessKeyBytes, ok := secret.Data[secretAccessKeyKey]
		if !ok {
			return nil, fmt.Errorf("secret access key not found in endpoint secret %s/%s with key %s", secretNamespace, endpoint.Spec.SecretRef.Name, secretAccessKeyKey)
		}
		config.SecretAccessKey = string(secretAccessKeyBytes)

	} else {
		return nil, fmt.Errorf("either AliasRef, URL, or EndpointRef must be specified")
	}

	// Handle credentials when using URL directly (not with AliasRef which handles its own credentials)
	if conn.URL != nil && conn.SecretRef != nil {
		// Get credentials from secret
		secretNamespace := defaultNamespace
		if conn.SecretRef.Namespace != nil {
			secretNamespace = *conn.SecretRef.Namespace
		}

		secret := &corev1.Secret{}
		err := k8sClient.Get(ctx, client.ObjectKey{
			Name:      conn.SecretRef.Name,
			Namespace: secretNamespace,
		}, secret)
		if err != nil {
			return nil, fmt.Errorf("failed to get secret %s/%s: %w", secretNamespace, conn.SecretRef.Name, err)
		}

		// Get access key ID
		accessKeyIDKey := conn.SecretRef.AccessKeyIDKey
		if accessKeyIDKey == "" {
			accessKeyIDKey = "accessKeyID"
		}
		accessKeyIDBytes, ok := secret.Data[accessKeyIDKey]
		if !ok {
			return nil, fmt.Errorf("access key ID not found in secret %s/%s with key %s", secretNamespace, conn.SecretRef.Name, accessKeyIDKey)
		}
		config.AccessKeyID = string(accessKeyIDBytes)

		// Get secret access key
		secretAccessKeyKey := conn.SecretRef.SecretAccessKeyKey
		if secretAccessKeyKey == "" {
			secretAccessKeyKey = "secretAccessKey"
		}
		secretAccessKeyBytes, ok := secret.Data[secretAccessKeyKey]
		if !ok {
			return nil, fmt.Errorf("secret access key not found in secret %s/%s with key %s", secretNamespace, conn.SecretRef.Name, secretAccessKeyKey)
		}
		config.SecretAccessKey = string(secretAccessKeyBytes)

		// Apply TLS config from connection if specified
		if conn.TLS != nil {
			config.Insecure = conn.TLS.Insecure
		}
	}

	return config, nil
}

// HealthCheck performs a health check on the MinIO server
func (c *Client) HealthCheck(ctx context.Context) error {
	// Try to list buckets as a simple health check
	_, err := c.S3.ListBuckets(ctx)
	return err
}

// GetServerInfo returns server information
func (c *Client) GetServerInfo(ctx context.Context) (madmin.InfoMessage, error) {
	return c.Admin.ServerInfo(ctx)
}

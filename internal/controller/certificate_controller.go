/*
Copyright 2024 PNarode.

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
	"github.com/PNarode/k8c-certs-manager/internal/helper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	certsv1 "github.com/PNarode/k8c-certs-manager/api/v1"
)

// CertificateReconciler reconciles a Certificate object
type CertificateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=certs.k8c.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=certs.k8c.io,resources=certificates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=certs.k8c.io,resources=certificates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Certificate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *CertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile Event: Certificate Reconcilation Started")
	// Fetch the Certificate instance
	certificate := &certsv1.Certificate{}
	err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, certificate)
	if err != nil {
		// Object not found, return. Created objects are automatically garbage collected.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch Request Type annotation from Certificate Request Object
	requestType, found := certificate.Annotations["requestType"]
	// No Request Type Annotation found. So perform normal Controller Reconcilation logic
	if !found {
		logger.Info("Reconcile Event: Attempting to check if Certificate Exists")
		secret := &corev1.Secret{}
		err = r.Get(ctx, types.NamespacedName{Name: certificate.Spec.SecretRef.Name, Namespace: req.Namespace}, secret)
		// Reconcile and Create missing secrets
		if err != nil {
			logger.Info("Reconcile Event: Certificate TLS secret reference does not exists", "Secret", certificate.Spec.SecretRef)
			logger.Info("Reconcile Event: Attempting to bring resource to desired state")
			err = r.createCertificate(ctx, *certificate, nil, req, "ReconileRequest")
			if err != nil {
				logger.Error(err, "Reconcile Event: Failed to bring resource to desired state")
				return ctrl.Result{}, client.IgnoreAlreadyExists(err)
			}
			return ctrl.Result{}, nil
		}

		// Reconcile and Check if Certificate Renewal is Required
		expiryDate := certificate.Status.ExpiryDate.Time
		renewBefore, _ := time.ParseDuration(certificate.Spec.RenewBefore)

		if time.Until(expiryDate) <= renewBefore {
			logger.Info("Reconcile Event: Renewing the certificate")
			err = r.renewCertificate(ctx, *certificate, secret, req)
			if err != nil {
				logger.Error(err, "Reconcile Event: Failed to renew certificate")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	}

	logger.Info("Reconcile Event: Event Detected", "RequestType", requestType)
	switch requestType {
	case "CreateRequest":
		logger.Info("Reconcile Create Event: Attempting to create new certificate and secret")
		err = r.createCertificate(ctx, *certificate, nil, req, "CreateRequest")
		if err != nil {
			if client.IgnoreAlreadyExists(err) != nil {
				return ctrl.Result{}, err
			}
			logger.Info("Reconcile Create Event: Failed as Certificate already exists")
		}
		delete(certificate.GetAnnotations(), "requestType")
		err = r.Update(ctx, certificate)
		if err != nil {
			logger.Error(err, "Reconcile Create Event: Failed to update certificate annotation")
			return ctrl.Result{}, err
		}
		logger.Info("Reconcile Create Event: Ended")
	case "UpdateRequest":
		logger.Info("Reconcile Update Event: Attempting to update certificate and secret")
		olderSecret := ""
		secret := &corev1.Secret{}
		err = r.Get(ctx, types.NamespacedName{Name: certificate.Spec.SecretRef.Name, Namespace: req.Namespace}, secret)
		if err != nil {
			logger.Info("Reconcile Update Event: Certificate TLS secret reference does not exists", "Secret", certificate.Spec.SecretRef)
			if certificate.Status.SecretRef != certificate.Spec.SecretRef.Name {
				logger.Info("Reconcile Update Event: TLS secret reference changed.")
				olderSecret = certificate.Status.SecretRef
			}
			logger.Info("Reconcile Update Event: Attempting to bring resource to desired state")
			err = r.createCertificate(ctx, *certificate, nil, req, "UpdateRequest")
			if err != nil {
				if client.IgnoreAlreadyExists(err) != nil {
					return ctrl.Result{}, err
				}
				logger.Info("Reconcile Update Event: Failed as Certificate already exists")
			}
			delete(certificate.GetAnnotations(), "requestType")
			err = r.Update(ctx, certificate)
			if err != nil {
				logger.Error(err, "Reconcile Update Event: Failed to update certificate annotation")
				return ctrl.Result{}, err
			}
			logger.Info("Reconcile Update Event: Certificate Desired State Achived")
			if olderSecret != "" {
				err = r.Get(ctx, types.NamespacedName{Name: olderSecret, Namespace: req.Namespace}, secret)
				if err == nil {
					logger.Info("Reconcile Update Event: cleanup of older secret")
					err = r.Delete(ctx, secret)
					if err != nil {
						logger.Error(err, "Reconcile Update Event: failed to clean older secret")
						if client.IgnoreNotFound(err) != nil {
							certificate.Annotations["requestType"] = "CleanupRequest"
							certificate.Annotations["deleteSecret"] = olderSecret
							err = r.Update(ctx, certificate)
							if err != nil {
								logger.Error(err, "Reconcile Update Event: Failed to update certificate annotation")
								return ctrl.Result{}, err
							}
							return ctrl.Result{}, err
						}
					}
				}
			}
		} else {
			logger.Info("Reconcile Update Event: Certificate TLS secret reference exists", "Secret", certificate.Spec.SecretRef)
			logger.Info("Reconcile Update Event: Attempting to bring resource to desired state")
			err = r.createCertificate(ctx, *certificate, secret, req, "UpdateRequest")
			if err != nil {
				logger.Error(err, "Reconcile Update Event: Failed to bring resource to desired state")
				if client.IgnoreAlreadyExists(err) != nil {
					return ctrl.Result{}, err
				}
				logger.Info("Reconcile Update Event: Failed as Certificate already exists")
			}
			logger.Info("Reconcile Update Event: Certificate Renewed")
		}
		logger.Info("Reconcile Update Event: Ended")
	case "CleanupRequest":
		logger.Info("Reconcile Cleanup Event: Attempting to delete older secret")
		secret := &corev1.Secret{}
		deleteSecret, found := certificate.Annotations["deleteSecret"]
		if found {
			err = r.Get(ctx, types.NamespacedName{Name: deleteSecret, Namespace: req.Namespace}, secret)
			if err == nil {
				logger.Info("Reconcile Cleanup Event: cleanup of older secret")
				err = r.Delete(ctx, secret)
				if err != nil {
					logger.Error(err, "Reconcile Cleanup Event: failed to cleanup older secret")
					return ctrl.Result{}, err
				}
			}
			delete(certificate.GetAnnotations(), "deleteSecret")
		} else {
			logger.Info("Reconcile Cleanup Event: No older secret found")
		}
		logger.Info("Reconcile Cleanup Event: Ended")
	}

	delete(certificate.GetAnnotations(), "requestType")
	err = r.Update(ctx, certificate)
	if err != nil {
		logger.Error(err, "Reconcile Event: Failed to update certificate annotation")
		return ctrl.Result{}, err
	}
	logger.Info("Reconcile Event: Certificate Reconcilation Ended")
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCert := e.ObjectOld.(*certsv1.Certificate)
			newCert := e.ObjectNew.(*certsv1.Certificate)
			return !reflect.DeepEqual(oldCert.Spec, newCert.Spec)
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&certsv1.Certificate{}).
		WithEventFilter(p).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func (r *CertificateReconciler) createCertificate(ctx context.Context, certificate certsv1.Certificate, secret *corev1.Secret, req ctrl.Request, reason string) error {
	logger := log.FromContext(ctx)

	// Generate a new self-signed certificate
	cert, key, err := helper.GenerateSelfSignedCertificate(certificate)
	if err != nil {
		logger.Error(err, "Failed to generate self-signed certificate")
		return err
	}

	if secret == nil {
		// Store the certificate and key in a Kubernetes Secret
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      certificate.Spec.SecretRef.Name,
				Namespace: req.Namespace,
			},
			Data: map[string][]byte{
				"tls.crt": cert,
				"tls.key": key,
			},
			Type: corev1.SecretTypeTLS,
		}

		// Create the secret in Kubernetes
		if err := r.Create(ctx, secret); err != nil {
			logger.Error(err, "Failed to create secret for TLS certificate")
			return err
		}
		logger.Info("TLS Certificate Issued Successfully", "Secret", certificate.Spec.SecretRef)
		err = r.updateStatus(ctx, &certificate, false)
		if err != nil {
			logger.Error(err, "Failed to update certificate status")
			return err
		}
	} else {
		secret.Data = map[string][]byte{
			"tls.crt": cert,
			"tls.key": key,
		}
		if err := r.Update(ctx, secret); err != nil {
			logger.Error(err, "Failed to update secret from updated TLS certificate")
			return err
		}
		logger.Info("TLS Certificate Updated Successfully", "Secret", certificate.Spec.SecretRef)
		err = r.updateStatus(ctx, &certificate, false)
		if err != nil {
			logger.Error(err, "Failed to update certificate status")
			return err
		}
	}
	return nil
}

func (r *CertificateReconciler) renewCertificate(ctx context.Context, certificate certsv1.Certificate, secret *corev1.Secret, req ctrl.Request) error {
	logger := log.FromContext(ctx)

	// Generate a new self-signed certificate
	cert, key, err := helper.GenerateSelfSignedCertificate(certificate)
	if err != nil {
		logger.Error(err, "Failed to renew self-signed certificate")
		return err
	}

	secret.Data = map[string][]byte{
		"tls.crt": cert,
		"tls.key": key,
	}

	if err := r.Update(ctx, secret); err != nil {
		logger.Error(err, "Failed to update secret from renewed TLS certificate")
		return err
	}
	logger.Info("TLS Certificate Renewed Successfully", "Secret", certificate.Spec.SecretRef)
	err = r.updateStatus(ctx, &certificate, true)
	if err != nil {
		logger.Error(err, "Failed to update certificate status")
		return err
	}
	return nil
}

func (r *CertificateReconciler) updateStatus(ctx context.Context, certificate *certsv1.Certificate, renewed bool) error {
	logger := log.FromContext(ctx)
	validity, _ := time.ParseDuration(certificate.Annotations["validityInHours"])
	certificate.Status.ExpiryDate = metav1.NewTime(time.Now().Add(validity))
	certificate.Status.SecretRef = certificate.Spec.SecretRef.Name
	certificate.Status.ObservedGeneration = certificate.Generation
	if renewed {
		certificate.Status.RenewedAt = metav1.NewTime(time.Now())
	}
	err := r.Status().Update(ctx, certificate)
	if err != nil {
		logger.Error(err, "Reconcile Update Event: Failed to update certificate status")
		return err
	}
	return nil
}

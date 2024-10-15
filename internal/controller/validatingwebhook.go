package controller

import (
	"context"
	"fmt"
	"github.com/PNarode/k8c-certs-manager/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"time"
)

// +kubebuilder:webhook:path=/validate-certs-k8c-io-v1-pod,mutating=true,failurePolicy=fail,sideEffects=None,groups="certs.k8c.io",resources=certificates,verbs=create;update,versions=v1,name=vcertificate.kb.io,admissionReviewVersions=v1

// CertificateValidator validates Certificate Resource
type CertificateValidator struct {
	client.Client
}

// validate admits a pod if a specific annotation exists.
func (v *CertificateValidator) validate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	log := logf.FromContext(ctx)
	// Check whether certificate mutation was triggered
	cert, ok := obj.(*v1.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected a Certificate but got a %T", obj)
	}

	log.Info("Validating Certificate Request")

	validityValue, found := cert.Annotations["validityInHours"]
	if !found {
		return nil, fmt.Errorf("invalid value %s for Validity field, should end with `h`(hours), `d`(days) or `y`(years) e:g 1y, 20d", cert.Spec.Validity)
	}
	_, err := time.ParseDuration(validityValue)
	if err != nil {
		return nil, fmt.Errorf("invalid value %s for Validity field, should end with `h`(hours), `d`(days) or `y`(years) e:g 1y, 20d", cert.Spec.Validity)
	}

	renewBefore, err := time.ParseDuration(cert.Spec.RenewBefore)
	if err != nil {
		return nil, fmt.Errorf("invalid value %s for RenewBefore field eg: 5m, 1d", cert.Spec.Validity)
	}
	if renewBefore < (5 * time.Minute) {
		return nil, fmt.Errorf("invalid value %s for RenewBefore field minimum value should be 5m", cert.Spec.Validity)
	}
	log.Info("Validation for Certificate Request Completed")
	return nil, nil
}

func (v *CertificateValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := logf.FromContext(ctx)
	cert, ok := obj.(*v1.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected a Certificate but got a %T", obj)
	}
	secret := &corev1.Secret{}
	err := v.Get(ctx, types.NamespacedName{Name: cert.Spec.SecretRef.Name, Namespace: cert.Namespace}, secret)
	if err == nil {
		logger.Info("TLS secret reference already exists", "Secret", cert.Spec.SecretRef)
		return nil, fmt.Errorf("TLS secret reference already exists")
	}
	cert.Annotations["requestType"] = "CreateRequest"
	return v.validate(ctx, obj)
}

func (v *CertificateValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logger := logf.FromContext(ctx)
	warn, err := v.validate(ctx, newObj)
	if err != nil {
		return warn, err
	}
	oldCert, ok := oldObj.(*v1.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected a Certificate but got a %T", oldObj)
	}
	newCert, ok := newObj.(*v1.Certificate)
	if !ok {
		return nil, fmt.Errorf("expected a Certificate but got a %T", oldObj)
	}
	if oldCert.Spec.SecretRef != newCert.Spec.SecretRef {
		secret := &corev1.Secret{}
		err = v.Get(ctx, types.NamespacedName{Name: oldCert.Spec.SecretRef.Name, Namespace: oldCert.Namespace}, secret)
		if err == nil {
			logger.Info("TLS secret reference already exists", "Secret", oldCert.Spec.SecretRef)
			logger.Info("Adding resource annotations to cleanup older secret")
			newCert.Annotations["deleteSecret"] = oldCert.Spec.SecretRef.Name
		}
	}
	newCert.Annotations["requestType"] = "UpdateRequest"
	return warn, err
}

func (v *CertificateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

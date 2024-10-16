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

// +kubebuilder:webhook:path=/validate-certs-k8c-io-v1-certificate,mutating=false,failurePolicy=fail,sideEffects=None,groups="certs.k8c.io",resources=certificates,verbs=create;update,versions=v1,name=vcertificate.kb.io,admissionReviewVersions=v1
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

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

	validityValue, found := cert.Annotations["validityInHours"]
	if !found {
		return nil, fmt.Errorf("no validity value annotations found")
	}
	_, err := time.ParseDuration(validityValue)
	if err != nil {
		return nil, fmt.Errorf("invalid value %s for Validity field err: %s", validityValue, err.Error())
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
	logger.Info("Validating Create Certificate Request")
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
	return v.validate(ctx, obj)
}

func (v *CertificateValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Validating Update Certificate Request")
	return v.validate(ctx, newObj)
}

func (v *CertificateValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

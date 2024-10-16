package controller

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/PNarode/k8c-certs-manager/api/v1"
	"math/big"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// +kubebuilder:webhook:path=/mutate-certs-k8c-io-v1-certificate,mutating=true,failurePolicy=fail,sideEffects=None,groups="certs.k8c.io",resources=certificates,verbs=create;update,versions=v1,name=mcertificate.kb.io,admissionReviewVersions=v1

// CertificateAnnotator annotates Certificate Resource
type CertificateAnnotator struct {
	client.Client
}

func (a *CertificateAnnotator) Default(ctx context.Context, obj runtime.Object) error {
	log := logf.FromContext(ctx)
	// Check whether certificate mutation was triggered
	cert, ok := obj.(*v1.Certificate)
	if !ok {
		return fmt.Errorf("expected a Certificate but got a %T", obj)
	}
	existingCert := &v1.Certificate{}
	err := a.Get(ctx, client.ObjectKey{
		Namespace: cert.Namespace,
		Name:      cert.Name,
	}, existingCert)
	if client.IgnoreNotFound(err) != nil {
		return err
	}
	if reflect.DeepEqual(cert.Spec, existingCert.Spec) {
		return nil
	}
	log.Info("Mutating Certificate Request")

	if cert.Annotations == nil {
		cert.Annotations = map[string]string{}
	}

	if cert.Spec.Subject == nil {
		serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if err != nil {
			log.Error(err, "failed to generate random serial number for certificate")
			serialNumber.SetString("123456789123456789123456789", 10)
		}

		cert.Spec.Subject = &v1.X509PkixSubject{
			Country:            []string{""},
			Organization:       []string{""},
			OrganizationalUnit: []string{""},
			CommonName:         cert.Spec.DNSName,
			SerialNumber:       serialNumber.String(),
		}
	}

	validityValue := cert.Spec.Validity
	switch validityValue[len(validityValue)-1:] {
	case "d":
		days, err := strconv.Atoi(strings.TrimSuffix(validityValue, "d"))
		if err != nil {
			log.Error(err, "failed to parse validity value for certificate")
			return fmt.Errorf("failed to parse validity value for certificate")
		}
		cert.Annotations["validityInHours"] = fmt.Sprintf("%vh", days*24)
	case "y":
		year, err := strconv.Atoi(strings.TrimSuffix(validityValue, "y"))
		if err != nil {
			log.Error(err, "failed to parse validity value for certificate")
			return fmt.Errorf("failed to parse validity value for certificate")
		}
		cert.Annotations["validityInHours"] = fmt.Sprintf("%vh", year*365*24)
	case "h":
		_, err := time.ParseDuration(validityValue)
		if err != nil {
			fmt.Println("Error:", err)
			return fmt.Errorf("invalid value %s for Validity field, should end with `h`(hours), `d`(days) or `y`(years) e:g 1y, 20d", cert.Spec.Validity)
		}
		cert.Annotations["validityInHours"] = validityValue
	}

	if cert.Spec.RenewBefore == "" {
		cert.Spec.RenewBefore = "5m"
	}

	cert.Annotations["requestType"] = "UpdateRequest"
	if cert.Status.ObservedGeneration == 0 {
		log.Info("create operation")
		cert.Annotations["requestType"] = "CreateRequest"
	}
	log.Info("Mutation for Certificate Completed")
	return nil
}

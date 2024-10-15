package helper

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	certsv1 "github.com/PNarode/k8c-certs-manager/api/v1"
	"time"
)

const (
	ConditionPending  string = "Pending"
	ConditionIssued   string = "Issued"
	ConditionRenewing string = "Renewing"
	ConditionRenewed  string = "Renewed"
	ConditionExpired  string = "Expired"
	ConditionFailed   string = "Failed"
)

// GenerateSelfSignedCertificate generates a new self-signed certificate
func GenerateSelfSignedCertificate(cert certsv1.Certificate) ([]byte, []byte, error) {
	// Create a new private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	// Create a template for the certificate
	notBefore := time.Now()
	validity, _ := time.ParseDuration(cert.Annotations["validityInHours"])
	notAfter := notBefore.Add(validity)

	details := cert.Spec
	subject := pkix.Name{
		Country:            details.Subject.Country,
		Organization:       details.Subject.Organization,
		OrganizationalUnit: details.Subject.OrganizationalUnit,
		SerialNumber:       details.Subject.SerialNumber,
		CommonName:         details.Subject.CommonName,
	}
	template := x509.Certificate{
		DNSNames:              []string{details.DNSName},
		EmailAddresses:        details.EmailAddresses,
		Subject:               subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  false,
		BasicConstraintsValid: true,
	}
	// Create a certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// Encode the certificate and key in PEM format
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return certPEM, keyPEM, nil
}

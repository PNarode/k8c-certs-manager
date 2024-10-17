package helper

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	certsv1 "github.com/PNarode/k8c-certs-manager/api/v1"
	"math/big"
	"time"
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
	commonName := details.DNSName
	if details.Subject.CommonName != "" {
		commonName = details.Subject.CommonName
	}
	subject := pkix.Name{
		Country:            details.Subject.Country,
		Organization:       details.Subject.Organization,
		OrganizationalUnit: details.Subject.OrganizationalUnit,
		SerialNumber:       details.Subject.SerialNumber,
		CommonName:         commonName,
	}
	template := x509.Certificate{
		DNSNames:              []string{details.DNSName},
		EmailAddresses:        details.EmailAddresses,
		Subject:               subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		SerialNumber:          new(big.Int),
		IsCA:                  false,
		BasicConstraintsValid: true,
	}
	template.SerialNumber.SetString(details.Subject.SerialNumber, 10)
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

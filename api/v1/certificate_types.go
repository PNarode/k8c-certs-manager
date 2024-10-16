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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretRef for specific secrets details
type SecretRef struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name,omitempty"`
}

// X509PkixSubject Full X509 name specification as per: https://pkg.go.dev/crypto/x509/pkix#Name
type X509PkixSubject struct {
	// Country to be used on the Certificate.
	// +optional
	Country []string `json:"country,omitempty"`
	// Organization to be used on the Certificate.
	// +optional
	Organization []string `json:"organization,omitempty"`
	// Organizational Unit to be used on the Certificate.
	// +optional
	OrganizationalUnit []string `json:"organizationalUnit,omitempty"`
	// Common Name to be used on the Certificate
	// +optional
	CommonName string `json:"commonName,omitempty"`
	// Serial number to be used on the Certificate.
	// +optional
	SerialNumber string `json:"serialNumber,omitempty"`
}

// CertificateSpec defines the desired state of Certificate
type CertificateSpec struct {
	// Requested set of X509 certificate subject attributes.
	// More info: https://pkg.go.dev/crypto/x509/pkix#Name
	//
	// Name represents an X.509 distinguished name.
	// Note that Name is only an approximation of the X.509 structure.
	Subject *X509PkixSubject `json:"subject,omitempty"`

	// Requested DNS subject alternative names.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	DNSName string `json:"dnsName,omitempty"`

	// Requested email subject alternative names.
	// +optional
	EmailAddresses []string `json:"emailAddresses,omitempty"`

	// Requested 'validity' (i.e. lifetime) of the Certificate.
	//
	// If unset, this defaults to 360 days.
	// Minimum accepted duration is 1 hour.
	// +kubebuilder:validation:Pattern=`^\d+[hdy]$`
	// +kubebuilder:validation:Required
	Validity string `json:"validity,omitempty"`

	// How long before the currently issued certificate's expiry cert-manager should
	// renew the certificate. For example, if a certificate is valid for 60 minutes,
	// and `renewBefore=10m`, cert-manager will begin to attempt to renew the certificate
	// 50 minutes after it was issued (i.e. when there are 10 minutes remaining until
	// the certificate is no longer valid).
	//
	// NOTE: The actual lifetime of the issued certificate is used to determine the
	// renewal time. If an issuer returns a certificate with a different lifetime than
	// the one requested, cert-manager will use the lifetime of the issued certificate.
	//
	// If unset, this defaults to 1/3 of the issued certificate's lifetime.
	// Minimum accepted value is 5 minutes.
	// Value must be in units accepted by Go time.ParseDuration https://golang.org/pkg/time/#ParseDuration.
	// Cannot be set if the `renewBeforePercentage` field is set.
	// +kubebuilder:validation:Pattern=`^\d+[mh]$`
	// +optional
	RenewBefore string `json:"renewBefore,omitempty"`

	// Name of the Secret resource that will be automatically created and
	// managed by this Certificate resource. It will be populated with a
	// private key and certificate, signed by the denoted issuer. The Secret
	// resource lives in the same namespace as the Certificate resource.
	// +kubebuilder:validation:Required
	SecretRef SecretRef `json:"secretRef"`
}

// CertificateStatus defines the observed state of Certificate
type CertificateStatus struct {
	ExpiryDate         metav1.Time `json:"expiryDate,omitempty"`
	RenewedAt          metav1.Time `json:"renewedAt,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
	SecretRef          string      `json:"secretRef,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Certificate is the Schema for the certificates API
type Certificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CertificateSpec   `json:"spec,omitempty"`
	Status CertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CertificateList contains a list of Certificate
type CertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Certificate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Certificate{}, &CertificateList{})
}

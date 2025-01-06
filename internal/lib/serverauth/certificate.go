package serverauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

type cleanup = func()

type CertificateService interface {
	GenerateCertificate(groupId string) (string, cleanup, error)
	GenerateCaCertificate() (string, cleanup, error)
}

type CertificateServiceImpl struct {
}

// GenerateCaCertificate implements CertificateService.
func (c *CertificateServiceImpl) GenerateCaCertificate() (string, cleanup, error) {
	// Generate CA cert and private key
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "svarog.jerkic.dev",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Generate CA private key
	caPrivKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	// Create CA certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		log.Fatal(err)
	}

	// Save CA certificate
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	tmpFile, err := os.CreateTemp("", "ca.crt")
	if err != nil {
		log.Fatal("Failed to create temp file for ca.crt", "err", err)
	}

	log.Debug("Writting ca.crt file", "location", tmpFile.Name())
	os.WriteFile(tmpFile.Name(), caPEM, 0644)

	return tmpFile.Name(), func() {
		os.Remove(tmpFile.Name())
	}, nil

}

// GenerateCertificate implements CertificateService.
func (c *CertificateServiceImpl) GenerateCertificate(groupId string) (string, cleanup, error) {
	panic("unimplemented")
}

var _ CertificateService = &CertificateServiceImpl{}

func NewCertificateService() CertificateService {
	return &CertificateServiceImpl{}
}

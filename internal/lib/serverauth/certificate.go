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
	"go.mongodb.org/mongo-driver/mongo"
)

type cleanup = func()

type CertificateService interface {
	GenerateCertificate(groupId string) (string, cleanup, error)
	GenerateCaCertificate() (string, cleanup, error)
	GetCaCertificate() (*x509.Certificate, error)
}

type CertificateServiceImpl struct {
	filesCollecton *mongo.Collection
}

// GetCaCertificate implements CertificateService.
func (c *CertificateServiceImpl) GetCaCertificate() (*x509.Certificate, error) {
	panic("unimplemented")
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
	// Generate cert and private key
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: groupId,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		IsCA:                  false,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	// Generate private key
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	caCert, err := c.GetCaCertificate()
	if err != nil {
		return "", func() {}, err
	}

	// Create certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &privKey.PublicKey, privKey)
	if err != nil {
		log.Fatal(err)
	}

	// Save certificate
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	tmpFile, err := os.CreateTemp("", groupId+".crt")
	if err != nil {
		log.Fatal("Failed to create temp file for "+groupId+".crt", "err", err)
	}

	log.Debug("Writting "+groupId+".crt file", "location", tmpFile.Name())
	os.WriteFile(tmpFile.Name(), certPEM, 0644)

	return tmpFile.Name(), func() {
		os.Remove(tmpFile.Name())
	}, nil

}

var _ CertificateService = &CertificateServiceImpl{}

func NewCertificateService(filesCollecton *mongo.Collection) CertificateService {
	if filesCollecton == nil {
		panic("No files collection")
	}

	return &CertificateServiceImpl{
		filesCollecton: filesCollecton,
	}
}

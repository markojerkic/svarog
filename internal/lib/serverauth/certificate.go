package serverauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"go.mongodb.org/mongo-driver/mongo"
)

type cleanup = func()

type CertificateService interface {
	GenerateCaCertificate() (string, cleanup, error)
	GenerateCertificate(groupId string) (string, cleanup, error)
	GetCaCertificate() (*x509.Certificate, error)
}

type CertificateServiceImpl struct {
	filesCollecton *mongo.Collection
}

// GetCaCertificate implements CertificateService.
func (c *CertificateServiceImpl) GetCaCertificate() (*x509.Certificate, error) {
	panic("unimplemented")
}

func (c *CertificateServiceImpl) GenerateCaCertificate() (string, cleanup, error) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		return "", nil, errors.Join(errors.New("Error creating temp dir"), err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Generate CA private key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", nil, errors.Join(errors.New("Error generating private key"), err)
	}

	// Create CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Create CA certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return "", nil, errors.Join(errors.New("Error creating certificate"), err)
	}

	// Save CA certificate
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPath := filepath.Join(tempDir, "ca.crt")
	if err := os.WriteFile(caPath, caPEM, 0644); err != nil {
		return "", nil, errors.Join(errors.New("Error writing certificate to file"), err)
	}

	log.Debug("CA certificate generated", "caPath", caPath, "caKey", caKey)

	return caPath, cleanup, nil
}

func (c *CertificateServiceImpl) GenerateCertificate(groupId string) (string, cleanup, error) {

	caCert, err := c.GetCaCertificate()
	if err != nil {
		return "", nil, errors.Join(errors.New("Error getting CA certificate"), err)
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		return "", nil, errors.Join(errors.New("Error creating temp dir"), err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	// Generate private key
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		cleanup()
		return "", nil, errors.Join(errors.New("Error generating private key"), err)
	}

	// Create certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: groupId,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// Create certificate
	caKey := caCert.PublicKey.(*ecdsa.PublicKey)
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &privKey.PublicKey, caKey)
	if err != nil {
		cleanup()
		return "", nil, errors.Join(errors.New("Error creating certificate"), err)
	}

	// Encode certificate and private key
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	privKeyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		cleanup()
		return "", nil, errors.Join(errors.New("Error marshaling private key"), err)
	}

	privKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privKeyBytes,
	})

	// Create combined PEM file
	pemPath := filepath.Join(tempDir, "cert.pem")
	pemFile, err := os.OpenFile(pemPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		cleanup()
		return "", nil, errors.Join(errors.New("Error creating PEM file"), err)
	}
	defer pemFile.Close()

	// Write both certificate and private key to the PEM file
	if _, err := pemFile.Write(certPEM); err != nil {
		cleanup()
		return "", nil, errors.Join(errors.New("Error writing certificate to PEM file"), err)
	}
	if _, err := pemFile.Write(privKeyPEM); err != nil {
		cleanup()
		return "", nil, errors.Join(errors.New("Error writing private key to PEM file"), err)
	}

	return pemPath, cleanup, nil
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

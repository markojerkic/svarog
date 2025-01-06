package serverauth

import (
	"context"
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
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/util"
	"go.mongodb.org/mongo-driver/mongo"
)

type cleanup = func()

type CertificateService interface {
	GenerateCaCertificate(ctx context.Context) error
	GenerateCertificate(groupId string) (string, cleanup, error)
	GetCaCertificate() (*x509.Certificate, *ecdsa.PrivateKey, error)
}

type CertificateServiceImpl struct {
	mongoClinet *mongo.Client
	fileService files.FileService
}

// GetCaCertificate implements CertificateService.
func (c *CertificateServiceImpl) GetCaCertificate(ctx context.Context) (*x509.Certificate, *ecdsa.PrivateKey, error) {

	certs, err := util.StartTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		caCert, err := c.fileService.GetFile(ctx, "ca.crt")
		if err != nil {
			return nil, errors.Join(errors.New("Failed getting ca.crt"), err)
		}
		caKey, err := c.fileService.GetFile(ctx, "ca.key")
		if err != nil {
			return nil, errors.Join(errors.New("Failed getting ca.key"), err)
		}

		return struct {
			caCert []byte
			caKey  []byte
		}{caCert: caCert, caKey: caKey}, nil

	}, c.mongoClinet)

	if err != nil {
		return nil, nil, err
	}

	caCert := certs.(struct{ caCert []byte }).caCert
	caKey := certs.(struct{ caKey []byte }).caKey

	block, _ := pem.Decode(caCert)
	if block == nil {
		return nil, nil, errors.New("failed to decode certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, errors.New("failed to parse certificate")
	}

	block, _ = pem.Decode(caKey)
	if block == nil {
		return nil, nil, errors.New("failed to decode key")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, errors.New("failed to parse key")
	}

	return cert, key, nil
}

func (c *CertificateServiceImpl) GenerateCaCertificate() error {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		return errors.Join(errors.New("Error creating temp dir"), err)
	}

	defer func() {
		os.RemoveAll(tempDir)
	}()

	// Generate CA private key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return errors.Join(errors.New("Error generating private key"), err)
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
		return errors.Join(errors.New("Error creating certificate"), err)
	}

	// Save CA certificate
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	caPath := filepath.Join(tempDir, "ca.crt")
	if err := os.WriteFile(caPath, caPEM, 0644); err != nil {
		return errors.Join(errors.New("Error writing certificate to file"), err)
	}

	// Save CA private key
	caKeyBytes, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return errors.Join(errors.New("Error marshaling private key"), err)
	}
	caKeyPath := filepath.Join(tempDir, "ca.key")
	if err := os.WriteFile(caKeyPath, caKeyBytes, 0600); err != nil {
		return errors.Join(errors.New("Error writing private key to file"), err)
	}

	log.Debug("CA certificate generated", "caPath", caPath, "caKey", caKey)

	return c.saveCaCrt(context.Background(), caPath, caKeyPath)
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

// Save files to mongodb
func (c *CertificateServiceImpl) saveCaCrt(ctx context.Context, certPath string, privateKeyPath string) error {
	_, err := util.StartTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		err := c.fileService.SaveFile(ctx, "ca.crt", certPath)
		if err != nil {
			return struct{}{}, errors.Join(errors.New("Failed saving ca.cert"), err)
		}
		err = c.fileService.SaveFile(ctx, "ca.key", privateKeyPath)
		if err != nil {
			return struct{}{}, errors.Join(errors.New("Failed saving ca.key"), err)
		}

		return struct{}{}, nil
	}, c.mongoClinet)
	return err
}

var _ CertificateService = &CertificateServiceImpl{}

func NewCertificateService(fileService files.FileService, mongoClinet *mongo.Client) CertificateService {
	return &CertificateServiceImpl{
		fileService: fileService,
		mongoClinet: mongoClinet,
	}
}

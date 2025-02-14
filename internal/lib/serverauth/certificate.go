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
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/util"
	"go.mongodb.org/mongo-driver/mongo"
)

type cleanup = func()
type certFiles struct {
	caCert []byte
	caKey  []byte
}

type CertificateService interface {
	GenerateCaCertificate(ctx context.Context) error
	GenerateCertificate(ctx context.Context, groupId string) (string, cleanup, error)
	GetCaCertificate(ctx context.Context) (*x509.Certificate, *ecdsa.PrivateKey, error)
	GetCertificatesZip(ctx context.Context, groupId string) (string, cleanup, error)
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
			log.Error("Failed getting ca.crt", "err", err)
			return nil, err
		}
		caKey, err := c.fileService.GetFile(ctx, "ca.key")
		if err != nil {
			log.Error("Failed getting ca.key", "err", err)
			return nil, err
		}

		return &certFiles{caCert: caCert, caKey: caKey}, nil

	}, c.mongoClinet)

	if err != nil {
		return nil, nil, err
	}

	certsStruct, ok := certs.(*certFiles)
	if !ok {
		log.Error("Failed to cast to certFiles")
		return nil, nil, errors.New("Failed to cast to certFiles")
	}
	caCert := certsStruct.caCert
	caKey := certsStruct.caKey

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

func (c *CertificateServiceImpl) GenerateCaCertificate(ctx context.Context) error {
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
	caKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: caKeyBytes,
	})
	caKeyPath := filepath.Join(tempDir, "ca.key")
	if err := os.WriteFile(caKeyPath, caKeyPEM, 0600); err != nil {
		return errors.Join(errors.New("Error writing private key to file"), err)
	}

	log.Debug("CA certificate generated", "caPath", caPath, "caKey", caKey)

	return c.saveCaCrt(context.Background(), caPath, caKeyPath)
}

func (c *CertificateServiceImpl) GenerateCertificate(ctx context.Context, groupId string) (string, cleanup, error) {
	if groupId == "" {
		return "", nil, errors.New("groupId is empty")
	}

	caCert, privateKey, err := c.GetCaCertificate(ctx)
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
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		DNSNames:    []string{"*"},
		IPAddresses: []net.IP{net.ParseIP("0.0.0.0"), net.ParseIP("::"), net.IPv6loopback},
	}

	// Create certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &privKey.PublicKey, privateKey)
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

// GetCertificatesZip implements CertificateService.
func (c *CertificateServiceImpl) GetCertificatesZip(ctx context.Context, groupId string) (string, cleanup, error) {
	if groupId == "" {
		return "", nil, errors.New("groupId is empty")
	}

	caCert, _, err := c.GetCaCertificate(ctx)
	if err != nil {
		return "", nil, errors.Join(errors.New("error getting CA certificate"), err)
	}
	certPath, certCleanup, err := c.GenerateCertificate(ctx, groupId)
	if err != nil {
		return "", nil, errors.Join(errors.New("error generating certificate"), err)
	}
	defer certCleanup()

	// write caCert to file
	tempDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		return "", nil, errors.Join(errors.New("error creating temp dir"), err)
	}

	caCertPath := filepath.Join(tempDir, "ca.crt")
	caCertFile, err := os.OpenFile(caCertPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return "", nil, errors.Join(errors.New("error creating ca.crt file"), err)
	}
	defer caCertFile.Close()

	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caCert.Raw,
	})

	if _, err := caCertFile.Write(caCertPEM); err != nil {
		return "", nil, errors.Join(errors.New("error writing ca.crt file"), err)
	}

	// zip files
	zipPath := filepath.Join(tempDir, "certs.zip")
	err = util.ZipFiles(zipPath, []string{certPath, caCertPath})
	if err != nil {
		return "", nil, errors.Join(errors.New("error zipping files"), err)
	}

	return zipPath, func() {
		certCleanup()
		err := os.RemoveAll(tempDir)
		if err != nil {
			log.Error("Failed to remove temp dir", "err", err)
		}
	}, nil
}

// Save files to mongodb
func (c *CertificateServiceImpl) saveCaCrt(ctx context.Context, certPath string, privateKeyPath string) error {
	_, err := util.StartTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		err := c.fileService.SaveFile(ctx, "ca.crt", certPath)
		if err != nil {
			log.Error("Failed saving ca.cert", "err", err)
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

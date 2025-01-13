package serverauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"time"

	"github.com/stretchr/testify/assert"
)

func (s *ServerauthSuite) TestGenerateCaCertificate() {
	t := s.T()
	ctx := context.Background()

	// Generate the certificate
	err := s.certificatesService.GenerateCaCertificate(ctx)
	assert.NoError(t, err)

	// Get and validate certificate
	cacertBytes, err := s.filesService.GetFile(ctx, "ca.crt")
	assert.NoError(t, err)
	assert.NotEmpty(t, cacertBytes)

	// Parse the certificate PEM block
	certBlock, _ := pem.Decode(cacertBytes)
	assert.NotNil(t, certBlock)
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	// Parse the X509 certificate
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	assert.NoError(t, err)

	// Validate certificate properties
	assert.True(t, cert.IsCA)
	assert.Equal(t, "CA", cert.Subject.CommonName)
	assert.True(t, cert.NotAfter.After(time.Now().AddDate(9, 11, 0))) // Should be valid for ~10 years
	assert.True(t, cert.NotBefore.Before(time.Now()))
	assert.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageCertSign, cert.KeyUsage)

	// Get and validate private key
	cakeyBytes, err := s.filesService.GetFile(ctx, "ca.key")
	assert.NoError(t, err)
	assert.NotEmpty(t, cakeyBytes)

	// Parse the private key PEM block
	keyBlock, _ := pem.Decode(cakeyBytes)
	assert.NotNil(t, keyBlock)
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)

	// Parse the EC private key
	privateKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	assert.NoError(t, err)
	assert.NotNil(t, privateKey)

	// Verify the public key matches
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).Curve, privateKey.Curve)
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).X, privateKey.PublicKey.X)
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).Y, privateKey.PublicKey.Y)
}

func (s *ServerauthSuite) TestGenerateCertificate() {
	t := s.T()
	ctx := context.Background()

	// First generate CA certificate
	err := s.certificatesService.GenerateCaCertificate(ctx)
	assert.NoError(t, err)

	// Generate a client certificate
	groupID := "test-group"
	certPath, cleanup, err := s.certificatesService.GenerateCertificate(ctx, groupID)
	assert.NoError(t, err)
	defer cleanup()

	// Read and parse the generated PEM file
	pemData, err := os.ReadFile(certPath)
	assert.NoError(t, err)

	// Parse certificate
	certBlock, rest := pem.Decode(pemData)
	assert.NotNil(t, certBlock)
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	assert.NoError(t, err)

	// Parse private key
	keyBlock, _ := pem.Decode(rest)
	assert.NotNil(t, keyBlock)
	assert.Equal(t, "EC PRIVATE KEY", keyBlock.Type)

	privKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	assert.NoError(t, err)

	// Validate certificate properties
	assert.Equal(t, groupID, cert.Subject.CommonName)
	assert.False(t, cert.IsCA)
	assert.True(t, cert.NotAfter.After(time.Now().AddDate(9, 11, 0)))
	assert.True(t, cert.NotBefore.Before(time.Now()))
	assert.Equal(t, x509.KeyUsageDigitalSignature, cert.KeyUsage)
	assert.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

	// Verify the public key matches the private key
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).Curve, privKey.Curve)
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).X, privKey.PublicKey.X)
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).Y, privKey.PublicKey.Y)
}

func (s *ServerauthSuite) TestGetCaCertificate() {
	t := s.T()
	ctx := context.Background()

	// First generate CA certificate
	err := s.certificatesService.GenerateCaCertificate(ctx)
	assert.NoError(t, err)

	// Get the CA certificate
	cert, key, err := s.certificatesService.GetCaCertificate(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.NotNil(t, key)

	// Validate certificate properties
	assert.True(t, cert.IsCA)
	assert.Equal(t, "CA", cert.Subject.CommonName)
	assert.True(t, cert.NotAfter.After(time.Now().AddDate(9, 11, 0)))
	assert.True(t, cert.NotBefore.Before(time.Now()))

	// Validate key properties
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).Curve, key.Curve)
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).X, key.PublicKey.X)
	assert.Equal(t, cert.PublicKey.(*ecdsa.PublicKey).Y, key.PublicKey.Y)
}

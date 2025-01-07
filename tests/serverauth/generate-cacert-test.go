package serverauth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/stretchr/testify/assert"
)

func (s *ServerauthSuite) TestGenerateCaCertificate() {
	t := s.T()

	// Generate the certificate
	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(t, err)

	// Get and validate certificate
	cacertBytes, err := s.filesService.GetFile(context.Background(), "ca.crt")
	assert.NoError(t, err)
	assert.NotEmpty(t, cacertBytes)

	// Parse the certificate PEM block
	certBlock, _ := pem.Decode(cacertBytes)
	assert.NotNil(t, certBlock)
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	// Parse the X509 certificate
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	assert.NoError(t, err)
	assert.NotNil(t, cert)

	// Validate certificate properties
	assert.True(t, cert.IsCA)
	assert.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign, cert.KeyUsage)

	// Get and validate private key
	cakeyBytes, err := s.filesService.GetFile(context.Background(), "ca.key")
	assert.NoError(t, err)
	assert.NotEmpty(t, cakeyBytes)

	// Parse the private key PEM block
	keyBlock, _ := pem.Decode(cakeyBytes)
	assert.NotNil(t, keyBlock)
	assert.Equal(t, "RSA PRIVATE KEY", keyBlock.Type)

	// Parse the RSA private key
	privateKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	assert.NoError(t, err)
	assert.NotNil(t, privateKey)

	// Validate the private key
	err = privateKey.Validate()
	assert.NoError(t, err)

	// Verify that the public key in the certificate matches the private key
	certPublicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	assert.True(t, ok)
	assert.Equal(t, certPublicKey.N, privateKey.N)
	assert.Equal(t, certPublicKey.E, privateKey.E)
}

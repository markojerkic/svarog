package serverauth

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/stretchr/testify/assert"
)

func (s *ServerauthSuite) TestGenerateCertZip() {
	t := s.T()
	ctx := context.Background()

	// Get zip file with ca.crt and cert.pem
	// Unzip the file and validate the contents

	err := s.certificatesService.GenerateCaCertificate(ctx)
	assert.NoError(t, err)

	groupId := "test-group"
	zipPath, cleanup, err := s.certificatesService.GetCertificatesZip(ctx, groupId)
	log.Debug("zipPath", "zipPath", zipPath)
	assert.NoError(t, err)
	defer cleanup()

	// Validate zip file

	// Unzip the file
	filesPath, err := util.Unzip(zipPath, filepath.Join(os.TempDir(), "certs"))
	assert.NoError(t, err)
	defer os.RemoveAll(filesPath)

	// Validate the contents
	caCertPath := filepath.Join(filesPath, "ca.crt")
	cacertBytes, err := os.ReadFile(caCertPath)
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

	certPath := filepath.Join(filesPath, "cert.pem")
	certBytes, err := os.ReadFile(certPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, certBytes)

	// Parse the certificate PEM block
	certBlock, _ = pem.Decode(certBytes)
	assert.NotNil(t, certBlock)
	assert.Equal(t, "CERTIFICATE", certBlock.Type)

	// Parse the X509 certificate
	cert, err = x509.ParseCertificate(certBlock.Bytes)
	assert.NoError(t, err)

	// Validate certificate properties
	assert.False(t, cert.IsCA)

}

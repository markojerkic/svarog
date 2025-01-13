package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"

	"github.com/charmbracelet/log"
)

func BuildCredentials(caCertPath string, certPath string) *tls.Config {
	cacertBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Fatal("Failed to read ca cert", "error", err)
	}
	certBlock, _ := pem.Decode(cacertBytes)
	cert, err := x509.ParseCertificate(certBlock.Bytes)

	caPool := x509.NewCertPool()
	caPool.AddCert(cert)

	clientCert, err := tls.LoadX509KeyPair(certPath, certPath)

	// Configure TLS
	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}
}

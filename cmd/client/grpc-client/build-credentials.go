package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/charmbracelet/log"
)

func BuildCredentials(caCertPath string, certPath string, serverName string) *tls.Config {
	cacertBytes, err := os.ReadFile(caCertPath)
	if err != nil {
		log.Fatal("Failed to read ca cert", "error", err)
	}

	caPool := x509.NewCertPool()
	if ok := caPool.AppendCertsFromPEM(cacertBytes); !ok {
		log.Fatal("Failed to add CA certificate to pool")
	}

	clientCert, err := tls.LoadX509KeyPair(certPath, certPath)
	if err != nil {
		log.Fatal("Failed to load client certificate", "error", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   serverName,
	}
}

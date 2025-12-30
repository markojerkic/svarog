package serverauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/grpcserver"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (s *ServerauthSuite) TestGrpcConnection_WrongCA() {
	log.SetLevel(log.DebugLevel)

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		log.Fatal("Failed to get free tcp port", "err", err)
	}

	env := types.ServerEnv{
		GrpcServerPort: randomFreePort,
	}
	logIngestChan := make(chan db.LogLineWithHost)

	grpcServer := grpcserver.NewGrpcServer(s.certificatesService, s.projectsService, env, logIngestChan)

	client := &mockClient{
		serverPort: randomFreePort,
	}

	go grpcServer.Start()
	defer grpcServer.Stop()

	tempDir, err := os.MkdirTemp("", "wrong-ca")
	assert.NoError(s.T(), err)
	defer os.RemoveAll(tempDir)

	wrongCAKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(s.T(), err)

	wrongCA := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "Wrong CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	_, err = x509.CreateCertificate(rand.Reader, wrongCA, wrongCA, &wrongCAKey.PublicKey, wrongCAKey)
	assert.NoError(s.T(), err)

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(s.T(), err)

	clientCert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "wrong-ca-client",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(10, 0, 0),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCert, wrongCA, &clientKey.PublicKey, wrongCAKey)
	assert.NoError(s.T(), err)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertBytes,
	})

	keyBytes, err := x509.MarshalECPrivateKey(clientKey)
	assert.NoError(s.T(), err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	certPath := filepath.Join(tempDir, "client.pem")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	assert.NoError(s.T(), err)
	certFile.Write(certPEM)
	certFile.Write(keyPEM)
	certFile.Close()

	wrongClientCert, err := tls.LoadX509KeyPair(certPath, certPath)
	assert.NoError(s.T(), err)

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{wrongClientCert},
		RootCAs:      caPool,
		ServerName:   "0.0.0.0",
	}

	line := &rpc.Backlog{}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
	assert.Error(s.T(), err)
}

func (s *ServerauthSuite) TestGrpcConnection_ExpiredCertificate() {
	log.SetLevel(log.DebugLevel)

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		log.Fatal("Failed to get free tcp port", "err", err)
	}

	env := types.ServerEnv{
		GrpcServerPort: randomFreePort,
	}
	logIngestChan := make(chan db.LogLineWithHost)

	grpcServer := grpcserver.NewGrpcServer(s.certificatesService, s.projectsService, env, logIngestChan)

	client := &mockClient{
		serverPort: randomFreePort,
	}

	go grpcServer.Start()
	defer grpcServer.Stop()

	tempDir, err := os.MkdirTemp("", "expired-cert")
	assert.NoError(s.T(), err)
	defer os.RemoveAll(tempDir)

	caCert, caKey, err := s.certificatesService.GetCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(s.T(), err)

	expiredCert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: "expired-client",
		},
		NotBefore:   time.Now().AddDate(0, 0, -2),
		NotAfter:    time.Now().AddDate(0, 0, -1),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		DNSNames:    []string{"0.0.0.0"},
	}

	expiredCertBytes, err := x509.CreateCertificate(rand.Reader, expiredCert, caCert, &clientKey.PublicKey, caKey)
	assert.NoError(s.T(), err)

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: expiredCertBytes,
	})

	keyBytes, err := x509.MarshalECPrivateKey(clientKey)
	assert.NoError(s.T(), err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	certPath := filepath.Join(tempDir, "expired.pem")
	certFile, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	assert.NoError(s.T(), err)
	certFile.Write(certPEM)
	certFile.Write(keyPEM)
	certFile.Close()

	expiredClientCert, err := tls.LoadX509KeyPair(certPath, certPath)
	assert.NoError(s.T(), err)

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{expiredClientCert},
		RootCAs:      caPool,
		ServerName:   "0.0.0.0",
	}

	line := &rpc.Backlog{}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
	assert.Error(s.T(), err)
}

package grpcclient

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/util"
)

func UnzipCredentials(zipFilePath string) (caCertPath string, certPath string) {
	tempDir, err := os.MkdirTemp("", "certs")
	if err != nil {
		log.Fatal("Failed to create temp dir", "error", err)
	}

	filesPath, err := util.Unzip(zipFilePath, tempDir)
	if err != nil {
		log.Fatal("Failed to unzip", "error", err)
	}

	// Assert ca.crt and cert.pem exist in the unzipped directory
	if _, err := os.Stat(filepath.Join(filesPath, "ca.crt")); err != nil {
		log.Fatal("ca.crt not found", "error", err)
	}
	if _, err := os.Stat(filepath.Join(filesPath, "cert.pem")); err != nil {
		log.Fatal("cert.pem not found", "error", err)
	}

	caCertPath = filepath.Join(filesPath, "ca.crt")
	certPath = filepath.Join(filesPath, "cert.pem")
	return caCertPath, certPath
}

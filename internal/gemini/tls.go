package gemini

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// initTLS initializes the TLS configuration
func (s *Server) initTLS() error {
	// Check if cert/key files exist
	certPath := s.config.TLS.CertPath
	keyPath := s.config.TLS.KeyPath

	// If no cert/key specified or auto-generate is enabled, generate self-signed
	if certPath == "" || keyPath == "" || s.config.TLS.AutoGenerate {
		return s.generateSelfSignedCert()
	}

	// Load existing certificate
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		// If loading fails, try to generate new cert
		fmt.Printf("Failed to load certificate, generating new one: %v\n", err)
		return s.generateSelfSignedCert()
	}

	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	return nil
}

// generateSelfSignedCert generates a self-signed certificate for testing/TOFU
func (s *Server) generateSelfSignedCert() error {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // Valid for 1 year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"nophr Gemini Server"},
			CommonName:   s.host,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{s.host},
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Build TLS config first so we can still start even if persisting fails
	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privateKey,
	}

	s.tlsConfig = &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	// Persist the cert if paths are provided and auto-generate is enabled; warn but continue on failure
	if s.config.TLS.AutoGenerate && s.config.TLS.CertPath != "" && s.config.TLS.KeyPath != "" {
		if err := s.persistSelfSignedCert(derBytes, privateKey); err != nil {
			fmt.Printf("Warning: could not persist self-signed Gemini certificate to %s: %v (using in-memory cert only)\n", filepath.Dir(s.config.TLS.CertPath), err)
		}
	}

	return nil
}

// persistSelfSignedCert writes the generated certificate and key to disk
func (s *Server) persistSelfSignedCert(derBytes []byte, privateKey *ecdsa.PrivateKey) error {
	certPath := s.config.TLS.CertPath
	keyPath := s.config.TLS.KeyPath

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Write certificate
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key
	keyOut, err := os.Create(keyPath)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	fmt.Printf("Generated self-signed certificate: %s\n", certPath)
	return nil
}

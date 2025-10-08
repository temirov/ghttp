package certificates

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

func parseCertificateFromPEM(pemBytes []byte) (*x509.Certificate, error) {
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, errors.New("invalid certificate pem encoding")
	}
	if pemBlock.Type != certificatePemBlockType {
		return nil, fmt.Errorf("unexpected pem block type %s", pemBlock.Type)
	}
	certificate, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return certificate, nil
}

func parseRSAPrivateKeyFromPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, errors.New("invalid private key pem encoding")
	}
	if pemBlock.Type != privateKeyPemBlockType {
		return nil, fmt.Errorf("unexpected pem block type %s", pemBlock.Type)
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

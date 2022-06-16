package main

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/QQGoblin/go-sdk/pkg/pkiutil"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"math"
	"math/big"
	"time"
)

func main() {

	notBefore, _ := time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	notAfter, _ := time.Parse("2006-01-02 15:04:05", "2170-01-01 00:00:00")
	serial, _ := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))

	certTmpl := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "front-proxy-client",
		},
		DNSNames:              nil,
		IPAddresses:           nil,
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	if err := NewClientCertAndKeyToDisk(certTmpl, "tls/", "ca", "front-proxy-client"); err != nil {
		klog.Fatal(err.Error())
	}
}

func NewClientCertAndKeyToDisk(certTmpl *x509.Certificate, pkiPath, caName, certName string) error {

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiPath, caName)
	if err != nil {
		return errors.Wrapf(err, "error load ca from disk: %s", err.Error())
	}

	key, err := pkiutil.NewPrivateKey(x509.RSA)
	if err != nil {
		return errors.Wrapf(err, "error create key: %s", err.Error())
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return errors.Wrapf(err, "error create certificate: %s", err.Error())
	}

	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return errors.Wrapf(err, "error parse certificate: %s", err.Error())
	}

	if err := pkiutil.WriteCertAndKey(pkiPath, certName, cert, key); err != nil {
		return errors.Wrapf(err, "error write certificate and key file: %s", err.Error())
	}

	return nil
}

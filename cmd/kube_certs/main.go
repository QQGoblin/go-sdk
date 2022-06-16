package main

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/QQGoblin/go-sdk/pkg/pkiutil"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"
	"math"
	"math/big"
	"path"
	"time"
)

func main() {
	if err := CreateClientCertAndKeyToDisk("tls", "ca", "kube-apiserver-kubelet-client", []string{pkiutil.SystemPrivilegedGroup}); err != nil {
		klog.Fatalf("create kube-apiserver-kubelet-client tls pki failed: %s", err.Error())
	}

	if err := CreateClientCertAndKeyToDisk("tls", "ca", "front-proxy-client", nil); err != nil {
		klog.Fatalf("create kube-apiserver-kubelet-client tls pki failed: %s", err.Error())
	}

	if err := CreateKubeconfig("tls", "ca", "https://kubernetes.lb", "https://127.0.0.1:6443", "node1"); err != nil {
		klog.Fatalf("create kubernetes config failed: %s", err.Error())
	}
}

func CreateSignedCertAndKey(commonName string, organization []string, caCert *x509.Certificate, caKey crypto.Signer, extKeyUsage []x509.ExtKeyUsage) (*x509.Certificate, crypto.Signer, error) {

	notBefore, _ := time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	notAfter, _ := time.Parse("2006-01-02 15:04:05", "2170-01-01 00:00:00")
	serial, _ := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))

	certTmpl := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	key, err := pkiutil.NewPrivateKey(x509.RSA)
	if err != nil {
		return nil, nil, err
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil

}
func CreateClientCertAndKeyToDisk(pkiPath, ca, certs string, organization []string) error {

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiPath, ca)
	if err != nil {
		return errors.Wrapf(err, "error load ca from disk: %s", err.Error())
	}

	cert, key, err := CreateSignedCertAndKey(certs, organization, caCert, caKey, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	if err != nil {
		return errors.Wrapf(err, "error create certificate and key: %s", err.Error())
	}
	if err := pkiutil.WriteCertAndKey(pkiPath, certs, cert, key); err != nil {
		return errors.Wrapf(err, "error write certificate and key file: %s", err.Error())
	}

	return nil
}
func CreateKubeconfig(pkiPath, caName, controlPlaneEndpoint, localAPIEndpoint, nodeName string) error {

	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(pkiPath, caName)
	if err != nil {
		return errors.Wrapf(err, "error load ca from disk: %s", err.Error())
	}

	configSpecs := pkiutil.GetKubeConfigSpecsBase(controlPlaneEndpoint, localAPIEndpoint, nodeName)

	for name, configSpec := range configSpecs {
		cert, key, err := CreateSignedCertAndKey(configSpec.ClientName, configSpec.ClientCertAuth.Organizations, caCert, caKey, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
		if err != nil {
			return errors.Wrapf(err, "error create certificate and key: %s", err.Error())
		}

		encodedClientKey, err := keyutil.MarshalPrivateKeyToPEM(key)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal private key to PEM")
		}
		kubeconfig := pkiutil.CreateWithCerts(
			configSpec.APIServer,
			"cluster.local",
			configSpec.ClientName,
			pkiutil.EncodeCertPEM(caCert),
			encodedClientKey,
			pkiutil.EncodeCertPEM(cert),
		)

		filename := path.Join(pkiPath, name)
		if err := clientcmd.WriteToFile(*kubeconfig, filename); err != nil {
			return errors.Wrapf(err, "error write kubeconfig file to disk: %s", err.Error())
		}
	}
	return nil
}

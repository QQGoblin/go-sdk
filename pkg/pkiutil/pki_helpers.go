/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pkiutil

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"k8s.io/klog/v2"
	"math"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

const (
	// PrivateKeyBlockType is a possible value for pem.Block.Type.
	PrivateKeyBlockType = "PRIVATE KEY"
	// PublicKeyBlockType is a possible value for pem.Block.Type.
	PublicKeyBlockType = "PUBLIC KEY"
	// CertificateBlockType is a possible value for pem.Block.Type.
	CertificateBlockType = "CERTIFICATE"
	// RSAPrivateKeyBlockType is a possible value for pem.Block.Type.
	RSAPrivateKeyBlockType = "RSA PRIVATE KEY"
	rsaKeySize             = 2048
	CertificateValidity    = time.Hour * 24 * 365
)

// CertConfig is a wrapper around certutil.Config extending it with PublicKeyAlgorithm.
type CertConfig struct {
	certutil.Config
	PublicKeyAlgorithm x509.PublicKeyAlgorithm
}

// NewCertificateAuthority creates new certificate and private key for the certificate authority
func NewCertificateAuthority(config *CertConfig) (*x509.Certificate, crypto.Signer, error) {
	key, err := NewPrivateKey(config.PublicKeyAlgorithm)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create private key while generating CA certificate")
	}

	cert, err := certutil.NewSelfSignedCACert(config.Config, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create self-signed CA certificate")
	}

	return cert, key, nil
}

// NewIntermediateCertificateAuthority creates new certificate and private key for an intermediate certificate authority
func NewIntermediateCertificateAuthority(parentCert *x509.Certificate, parentKey crypto.Signer, config *CertConfig) (*x509.Certificate, crypto.Signer, error) {
	key, err := NewPrivateKey(config.PublicKeyAlgorithm)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create private key while generating intermediate CA certificate")
	}

	cert, err := NewSignedCert(config, key, parentCert, parentKey, true)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to sign intermediate CA certificate")
	}

	return cert, key, nil
}

// NewCertAndKey creates new certificate and key by passing the certificate authority certificate and key
func NewCertAndKey(caCert *x509.Certificate, caKey crypto.Signer, config *CertConfig) (*x509.Certificate, crypto.Signer, error) {
	if len(config.Usages) == 0 {
		return nil, nil, errors.New("must specify at least one ExtKeyUsage")
	}

	key, err := NewPrivateKey(config.PublicKeyAlgorithm)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create private key")
	}

	cert, err := NewSignedCert(config, key, caCert, caKey, false)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to sign certificate")
	}

	return cert, key, nil
}

// NewCSRAndKey generates a new key and CSR and that could be signed to create the given certificate
func NewCSRAndKey(config *CertConfig) (*x509.CertificateRequest, crypto.Signer, error) {
	key, err := NewPrivateKey(config.PublicKeyAlgorithm)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to create private key")
	}

	csr, err := NewCSR(*config, key)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to generate CSR")
	}

	return csr, key, nil
}

// HasServerAuth returns true if the given certificate is a ServerAuth
func HasServerAuth(cert *x509.Certificate) bool {
	for i := range cert.ExtKeyUsage {
		if cert.ExtKeyUsage[i] == x509.ExtKeyUsageServerAuth {
			return true
		}
	}
	return false
}

// WriteCertAndKey stores certificate and key at the specified location
func WriteCertAndKey(pkiPath string, name string, cert *x509.Certificate, key crypto.Signer) error {
	if err := WriteKey(pkiPath, name, key); err != nil {
		return errors.Wrap(err, "couldn't write key")
	}

	return WriteCert(pkiPath, name, cert)
}

// WriteCert stores the given certificate at the given location
func WriteCert(pkiPath, name string, cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("certificate cannot be nil when writing to file")
	}

	certificatePath := pathForCert(pkiPath, name)
	if err := certutil.WriteCert(certificatePath, EncodeCertPEM(cert)); err != nil {
		return errors.Wrapf(err, "unable to write certificate to file %s", certificatePath)
	}

	return nil
}

// WriteCertBundle stores the given certificate bundle at the given location
func WriteCertBundle(pkiPath, name string, certs []*x509.Certificate) error {
	for i, cert := range certs {
		if cert == nil {
			return errors.Errorf("found nil certificate at position %d when writing bundle to file", i)
		}
	}

	certificatePath := pathForCert(pkiPath, name)
	encoded, err := EncodeCertBundlePEM(certs)
	if err != nil {
		return errors.Wrapf(err, "unable to marshal certificate bundle to PEM")
	}
	if err := certutil.WriteCert(certificatePath, encoded); err != nil {
		return errors.Wrapf(err, "unable to write certificate bundle to file %s", certificatePath)
	}

	return nil
}

// WriteKey stores the given key at the given location
func WriteKey(pkiPath, name string, key crypto.Signer) error {
	if key == nil {
		return errors.New("private key cannot be nil when writing to file")
	}

	privateKeyPath := pathForKey(pkiPath, name)
	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return errors.Wrapf(err, "unable to marshal private key to PEM")
	}
	if err := keyutil.WriteKey(privateKeyPath, encoded); err != nil {
		return errors.Wrapf(err, "unable to write private key to file %s", privateKeyPath)
	}

	return nil
}

// WriteCSR writes the pem-encoded CSR data to csrPath.
// The CSR file will be created with file mode 0600.
// If the CSR file already exists, it will be overwritten.
// The parent directory of the csrPath will be created as needed with file mode 0700.
func WriteCSR(csrDir, name string, csr *x509.CertificateRequest) error {
	if csr == nil {
		return errors.New("certificate request cannot be nil when writing to file")
	}

	csrPath := pathForCSR(csrDir, name)
	if err := os.MkdirAll(filepath.Dir(csrPath), os.FileMode(0700)); err != nil {
		return errors.Wrapf(err, "failed to make directory %s", filepath.Dir(csrPath))
	}

	if err := ioutil.WriteFile(csrPath, EncodeCSRPEM(csr), os.FileMode(0600)); err != nil {
		return errors.Wrapf(err, "unable to write CSR to file %s", csrPath)
	}

	return nil
}

// WritePublicKey stores the given public key at the given location
func WritePublicKey(pkiPath, name string, key crypto.PublicKey) error {
	if key == nil {
		return errors.New("public key cannot be nil when writing to file")
	}

	publicKeyBytes, err := EncodePublicKeyPEM(key)
	if err != nil {
		return err
	}
	publicKeyPath := pathForPublicKey(pkiPath, name)
	if err := keyutil.WriteKey(publicKeyPath, publicKeyBytes); err != nil {
		return errors.Wrapf(err, "unable to write public key to file %s", publicKeyPath)
	}

	return nil
}

// CertOrKeyExist returns a boolean whether the cert or the key exists
func CertOrKeyExist(pkiPath, name string) bool {
	certificatePath, privateKeyPath := PathsForCertAndKey(pkiPath, name)

	_, certErr := os.Stat(certificatePath)
	_, keyErr := os.Stat(privateKeyPath)
	if os.IsNotExist(certErr) && os.IsNotExist(keyErr) {
		// The cert and the key do not exist
		return false
	}

	// Both files exist or one of them
	return true
}

// CSROrKeyExist returns true if one of the CSR or key exists
func CSROrKeyExist(csrDir, name string) bool {
	csrPath := pathForCSR(csrDir, name)
	keyPath := pathForKey(csrDir, name)

	_, csrErr := os.Stat(csrPath)
	_, keyErr := os.Stat(keyPath)

	return !(os.IsNotExist(csrErr) && os.IsNotExist(keyErr))
}

// TryLoadCertAndKeyFromDisk tries to load a cert and a key from the disk and validates that they are valid
func TryLoadCertAndKeyFromDisk(pkiPath, name string) (*x509.Certificate, crypto.Signer, error) {
	cert, err := TryLoadCertFromDisk(pkiPath, name)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load certificate")
	}

	key, err := TryLoadKeyFromDisk(pkiPath, name)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to load key")
	}

	return cert, key, nil
}

// TryLoadCertFromDisk tries to load the cert from the disk
func TryLoadCertFromDisk(pkiPath, name string) (*x509.Certificate, error) {
	certificatePath := pathForCert(pkiPath, name)

	certs, err := certutil.CertsFromFile(certificatePath)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't load the certificate file %s", certificatePath)
	}

	// We are only putting one certificate in the certificate pem file, so it's safe to just pick the first one
	// TODO: Support multiple certs here in order to be able to rotate certs
	cert := certs[0]

	return cert, nil
}

// TryLoadCertChainFromDisk tries to load the cert chain from the disk
func TryLoadCertChainFromDisk(pkiPath, name string) (*x509.Certificate, []*x509.Certificate, error) {
	certificatePath := pathForCert(pkiPath, name)

	certs, err := certutil.CertsFromFile(certificatePath)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't load the certificate file %s", certificatePath)
	}

	cert := certs[0]
	intermediates := certs[1:]

	return cert, intermediates, nil
}

// TryLoadKeyFromDisk tries to load the key from the disk and validates that it is valid
func TryLoadKeyFromDisk(pkiPath, name string) (crypto.Signer, error) {
	privateKeyPath := pathForKey(pkiPath, name)

	// Parse the private key from a file
	privKey, err := keyutil.PrivateKeyFromFile(privateKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't load the private key file %s", privateKeyPath)
	}

	// Allow RSA and ECDSA formats only
	var key crypto.Signer
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		key = k
	case *ecdsa.PrivateKey:
		key = k
	default:
		return nil, errors.Errorf("the private key file %s is neither in RSA nor ECDSA format", privateKeyPath)
	}

	return key, nil
}

// TryLoadCSRAndKeyFromDisk tries to load the CSR and key from the disk
func TryLoadCSRAndKeyFromDisk(pkiPath, name string) (*x509.CertificateRequest, crypto.Signer, error) {
	csr, err := TryLoadCSRFromDisk(pkiPath, name)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not load CSR file")
	}

	key, err := TryLoadKeyFromDisk(pkiPath, name)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not load key file")
	}

	return csr, key, nil
}

// TryLoadPrivatePublicKeyFromDisk tries to load the key from the disk and validates that it is valid
func TryLoadPrivatePublicKeyFromDisk(pkiPath, name string) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKeyPath := pathForKey(pkiPath, name)

	// Parse the private key from a file
	privKey, err := keyutil.PrivateKeyFromFile(privateKeyPath)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't load the private key file %s", privateKeyPath)
	}

	publicKeyPath := pathForPublicKey(pkiPath, name)

	// Parse the public key from a file
	pubKeys, err := keyutil.PublicKeysFromFile(publicKeyPath)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't load the public key file %s", publicKeyPath)
	}

	// Allow RSA format only
	k, ok := privKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, errors.Errorf("the private key file %s isn't in RSA format", privateKeyPath)
	}

	p := pubKeys[0].(*rsa.PublicKey)

	return k, p, nil
}

// TryLoadCSRFromDisk tries to load the CSR from the disk
func TryLoadCSRFromDisk(pkiPath, name string) (*x509.CertificateRequest, error) {
	csrPath := pathForCSR(pkiPath, name)

	csr, err := CertificateRequestFromFile(csrPath)
	if err != nil {
		return nil, errors.Wrapf(err, "could not load the CSR %s", csrPath)
	}

	return csr, nil
}

// PathsForCertAndKey returns the paths for the certificate and key given the path and basename.
func PathsForCertAndKey(pkiPath, name string) (string, string) {
	return pathForCert(pkiPath, name), pathForKey(pkiPath, name)
}

func pathForCert(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.crt", name))
}

func pathForKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.key", name))
}

func pathForPublicKey(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.pub", name))
}

func pathForCSR(pkiPath, name string) string {
	return filepath.Join(pkiPath, fmt.Sprintf("%s.csr", name))
}

// appendSANsToAltNames parses SANs from as list of strings and adds them to altNames for use on a specific cert
// altNames is passed in with a pointer, and the struct is modified
// valid IP address strings are parsed and added to altNames.IPs as net.IP's
// RFC-1123 compliant DNS strings are added to altNames.DNSNames as strings
// RFC-1123 compliant wildcard DNS strings are added to altNames.DNSNames as strings
// certNames is used to print user facing warnings and should be the name of the cert the altNames will be used for
func AppendSANsToAltNames(altNames *certutil.AltNames, SANs []string, certName string) {
	for _, altname := range SANs {
		if ip := net.ParseIP(altname); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		} else if len(validation.IsDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		} else if len(validation.IsWildcardDNS1123Subdomain(altname)) == 0 {
			altNames.DNSNames = append(altNames.DNSNames, altname)
		} else {
			fmt.Printf(
				"[certificates] WARNING: '%s' was not added to the '%s' SAN, because it is not a valid IP or RFC-1123 compliant DNS entry\n",
				altname,
				certName,
			)
		}
	}
}

// EncodeCSRPEM returns PEM-encoded CSR data
func EncodeCSRPEM(csr *x509.CertificateRequest) []byte {
	block := pem.Block{
		Type:  certutil.CertificateRequestBlockType,
		Bytes: csr.Raw,
	}
	return pem.EncodeToMemory(&block)
}

func parseCSRPEM(pemCSR []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(pemCSR)
	if block == nil {
		return nil, errors.New("data doesn't contain a valid certificate request")
	}

	if block.Type != certutil.CertificateRequestBlockType {
		return nil, errors.Errorf("expected block type %q, but PEM had type %q", certutil.CertificateRequestBlockType, block.Type)
	}

	return x509.ParseCertificateRequest(block.Bytes)
}

// CertificateRequestFromFile returns the CertificateRequest from a given PEM-encoded file.
// Returns an error if the file could not be read or if the CSR could not be parsed.
func CertificateRequestFromFile(file string) (*x509.CertificateRequest, error) {
	pemBlock, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	csr, err := parseCSRPEM(pemBlock)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading certificate request file %s", file)
	}
	return csr, nil
}

// NewCSR creates a new CSR
func NewCSR(cfg CertConfig, key crypto.Signer) (*x509.CertificateRequest, error) {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:    cfg.AltNames.DNSNames,
		IPAddresses: cfg.AltNames.IPs,
	}

	csrBytes, err := x509.CreateCertificateRequest(cryptorand.Reader, template, key)

	if err != nil {
		return nil, errors.Wrap(err, "failed to create a CSR")
	}

	return x509.ParseCertificateRequest(csrBytes)
}

// EncodeCertPEM returns PEM-endcoded certificate data
func EncodeCertPEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  CertificateBlockType,
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}

// EncodeCertBundlePEM returns PEM-endcoded certificate bundle
func EncodeCertBundlePEM(certs []*x509.Certificate) ([]byte, error) {
	buf := bytes.Buffer{}

	block := pem.Block{
		Type: CertificateBlockType,
	}

	for _, cert := range certs {
		block.Bytes = cert.Raw
		if err := pem.Encode(&buf, &block); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// EncodePublicKeyPEM returns PEM-encoded public data
func EncodePublicKeyPEM(key crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return []byte{}, err
	}
	block := pem.Block{
		Type:  PublicKeyBlockType,
		Bytes: der,
	}
	return pem.EncodeToMemory(&block), nil
}

// NewPrivateKey returns a new private key.
var NewPrivateKey = GeneratePrivateKey

func GeneratePrivateKey(keyType x509.PublicKeyAlgorithm) (crypto.Signer, error) {
	if keyType == x509.ECDSA {
		return ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	}

	return rsa.GenerateKey(cryptorand.Reader, rsaKeySize)
}

// NewSignedCert creates a signed certificate using the given CA certificate and key
func NewSignedCert(cfg *CertConfig, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer, isCA bool) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}

	keyUsage := x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature
	if isCA {
		keyUsage |= x509.KeyUsageCertSign
	}

	RemoveDuplicateAltNames(&cfg.AltNames)

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:              cfg.AltNames.DNSNames,
		IPAddresses:           cfg.AltNames.IPs,
		SerialNumber:          serial,
		NotBefore:             caCert.NotBefore,
		NotAfter:              time.Now().Add(CertificateValidity).UTC(),
		KeyUsage:              keyUsage,
		ExtKeyUsage:           cfg.Usages,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

// RemoveDuplicateAltNames removes duplicate items in altNames.
func RemoveDuplicateAltNames(altNames *certutil.AltNames) {
	if altNames == nil {
		return
	}

	if altNames.DNSNames != nil {
		altNames.DNSNames = sets.NewString(altNames.DNSNames...).List()
	}

	ipsKeys := make(map[string]struct{})
	var ips []net.IP
	for _, one := range altNames.IPs {
		if _, ok := ipsKeys[one.String()]; !ok {
			ipsKeys[one.String()] = struct{}{}
			ips = append(ips, one)
		}
	}
	altNames.IPs = ips
}

// ValidateCertPeriod checks if the certificate is valid relative to the current time
// (+/- offset)
func ValidateCertPeriod(cert *x509.Certificate, offset time.Duration) error {
	period := fmt.Sprintf("NotBefore: %v, NotAfter: %v", cert.NotBefore, cert.NotAfter)
	now := time.Now().Add(offset)
	if now.Before(cert.NotBefore) {
		return errors.Errorf("the certificate is not valid yet: %s", period)
	}
	if now.After(cert.NotAfter) {
		return errors.Errorf("the certificate has expired: %s", period)
	}
	return nil
}

// VerifyCertChain verifies that a certificate has a valid chain of
// intermediate CAs back to the root CA
func VerifyCertChain(cert *x509.Certificate, intermediates []*x509.Certificate, root *x509.Certificate) error {
	rootPool := x509.NewCertPool()
	rootPool.AddCert(root)

	intermediatePool := x509.NewCertPool()
	for _, c := range intermediates {
		intermediatePool.AddCert(c)
	}

	verifyOptions := x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: intermediatePool,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}

	if _, err := cert.Verify(verifyOptions); err != nil {
		return err
	}

	return nil
}

// TryLoadKeyFromString load key from string
func TryLoadKeyFromString(data string) (crypto.Signer, error) {

	privKey, err := keyutil.ParsePrivateKeyPEM([]byte(data))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %v", err)
	}

	// Allow RSA and ECDSA formats only
	var key crypto.Signer
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		key = k
	case *ecdsa.PrivateKey:
		key = k
	default:
		return nil, errors.New("the private key is neither in RSA nor ECDSA format")
	}

	return key, nil
}

// TryLoadKeyFromString load Cert from string
func TryLoadCertificateFromString(data string) (*x509.Certificate, error) {
	// 读取内置的证书文件
	certs, err := certutil.ParseCertsPEM([]byte(data))
	if err != nil {
		klog.Errorf("error parse bundle cacert: %s", err)
		return nil, err
	}
	return certs[0], nil
}

func LoadCertificateAndKeyFromString(cert, key string) (*x509.Certificate, crypto.Signer, error) {
	// 读取内置的证书文件
	caCert, err := TryLoadCertificateFromString(cert)
	if err != nil {
		klog.Errorf("load ca cert failed:%v", err)
		return nil, nil, err
	}

	caKey, err := TryLoadKeyFromString(key)
	if err != nil {
		klog.Errorf("load key failed:%v", err)
		return nil, nil, err
	}
	return caCert, caKey, nil
}

//LoadTLSCertificate 从 certStr、keyStr 获取 tls.Certificate 信息，用于 HTTPs 请求
func LoadTLSCertificate(certStr, keyStr string) (*tls.Certificate, error) {

	cert, err := tls.X509KeyPair([]byte(certStr), []byte(keyStr))
	if err != nil {
		klog.Errorf("error parse bundle cert and key: %s", err)
		return nil, err
	}

	return &cert, nil
}

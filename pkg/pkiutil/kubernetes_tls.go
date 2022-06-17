package pkiutil

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/keyutil"
	"math"
	"math/big"
	"net"
	"path"
	"time"
)

const (
	CACertAndKeyBaseName                     = "ca"
	EtcdCACertAndKeyBaseName                 = "etcd-ca"
	FrontProxyCACertAndKeyBaseName           = "front-proxy-ca"
	EtcdServerCertAndKeyBaseName             = "etcd-server"
	EtcdPeerCertAndKeyBaseName               = "etcd-peer"
	EtcdHealthcheckClientCertAndKeyBaseName  = "etcd-healthcheck-client"
	APIServerCertAndKeyBaseName              = "apiserver"
	APIServerKubeletClientCertAndKeyBaseName = "apiserver-kubelet-client"
	APIServerEtcdClientCertAndKeyBaseName    = "apiserver-etcd-client"
	FrontProxyClientCertAndKeyBaseName       = "front-proxy-client"
	ServiceAccountKeyBaseName                = "sa"

	AdminKubeConfigBaseName             = "admin.conf"
	ControllerManagerKubeConfigBaseName = "controller-manager.conf"
	SchedulerKubeConfigBaseName         = "scheduler.conf"
	KubeletKubeConfigBaseName           = "kubelet.conf"
	KubeProxyKubeConfigBaseName         = "kube-proxy.conf"

	APIServerCertCommonName              = "kube-apiserver"
	APIServerKubeletClientCertCommonName = "kube-apiserver-kubelet-client"
	EtcdHealthcheckClientCertCommonName  = "kube-etcd-healthcheck-client"
	APIServerEtcdClientCertCommonName    = "kube-apiserver-etcd-client"
	FrontProxyClientCertCommonName       = "front-proxy-client"
)

func GetCertificateTemplates(ctrlip, nodeip, svcip string, nodename string) (map[string]*x509.Certificate, error) {

	// TODO: 此处省略  etcd 相关证书，以及 front-proxy-ca

	notBefore, _ := time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	notAfter, _ := time.Parse("2006-01-02 15:04:05", "2170-01-01 00:00:00")
	serial, _ := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))

	return map[string]*x509.Certificate{
		CACertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: "kubernetes",
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		},
		EtcdCACertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: "etcd-ca",
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		},
		FrontProxyCACertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: "front-proxy-ca",
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		},
		EtcdServerCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: nodename,
			},
			IPAddresses: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP(nodeip),
			},
			DNSNames: []string{
				"localhost",
				nodename,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		EtcdPeerCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: nodename,
			},
			IPAddresses: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP(nodeip),
			},
			DNSNames: []string{
				"localhost",
				nodename,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		EtcdHealthcheckClientCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: EtcdHealthcheckClientCertCommonName,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		APIServerCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: APIServerCertCommonName,
			},
			IPAddresses: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP(ctrlip),
				net.ParseIP(nodeip),
				net.ParseIP(svcip),
			},
			DNSNames: []string{
				"localhost",
				"kubernetes",
				"kubernetes.default",
				"kubernetes.default.svc",
				"kubernetes.default.svc.cluster",
				"kubernetes.default.svc.cluster.local",
				nodename,
				"*.kubernetes.master",
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		APIServerEtcdClientCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName:   APIServerEtcdClientCertCommonName,
				Organization: []string{SystemPrivilegedGroup},
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		APIServerKubeletClientCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName:   APIServerKubeletClientCertCommonName,
				Organization: []string{SystemPrivilegedGroup},
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		FrontProxyClientCertAndKeyBaseName: {
			Subject: pkix.Name{
				CommonName: FrontProxyClientCertCommonName,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		AdminKubeConfigBaseName: {
			Subject: pkix.Name{
				CommonName:   "kubernetes-admin",
				Organization: []string{SystemPrivilegedGroup},
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		KubeletKubeConfigBaseName: {
			Subject: pkix.Name{
				CommonName:   fmt.Sprintf("%s%s", NodesUserPrefix, nodename),
				Organization: []string{NodesGroup},
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		ControllerManagerKubeConfigBaseName: {
			Subject: pkix.Name{
				CommonName: ControllerManagerUser,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		SchedulerKubeConfigBaseName: {
			Subject: pkix.Name{
				CommonName: SchedulerUser,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
		KubeProxyKubeConfigBaseName: {
			Subject: pkix.Name{
				CommonName: ProxyUser,
			},
			SerialNumber:          serial,
			NotBefore:             notBefore,
			NotAfter:              notAfter,
			KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			BasicConstraintsValid: true,
			IsCA:                  false,
		},
	}, nil
}

func CreateSignedCertAndKey(certTmpl *x509.Certificate, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, crypto.Signer, error) {

	key, err := NewPrivateKey(x509.RSA)
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

func CreateKubeConfigFromCertificate(caCert, cert *x509.Certificate, key crypto.Signer, apiserver string) (*clientcmdapi.Config, error) {

	encodedClientKey, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal private key to PEM")
	}
	kubeconfig := CreateWithCerts(
		apiserver,
		"cluster.local",
		cert.Subject.CommonName,
		EncodeCertPEM(caCert),
		encodedClientKey,
		EncodeCertPEM(cert),
	)

	return kubeconfig, nil
}

func GenerateServiceAccountKeyAndPublicKeyFiles(certsDir string, keyType x509.PublicKeyAlgorithm) error {
	key, err := NewPrivateKey(keyType)
	if err != nil {
		return err
	}
	if err := WriteKey(certsDir, ServiceAccountKeyBaseName, key); err != nil {
		return err
	}
	return WritePublicKey(certsDir, ServiceAccountKeyBaseName, key.Public())
}

func GenerateCertificateFiles(pkiPath, caName, cerName string, certTempl *x509.Certificate) error {

	caCert, caKey, err := TryLoadCertAndKeyFromDisk(pkiPath, caName)
	if err != nil {
		return errors.Wrap(err, "failed load ca from disk")
	}

	cert, key, err := CreateSignedCertAndKey(certTempl, caCert, caKey)
	if err != nil {
		return errors.Wrapf(err, "create cert and key for failed: %s", cerName)
	}
	if err := WriteCertAndKey(pkiPath, cerName, cert, key); err != nil {
		return errors.Wrapf(err, "error write certificate and key file: %s", cerName)
	}

	return nil
}

func GenerateKubeConfigFiles(pkiPath, caName, name string, certTempl *x509.Certificate, apiserver string) error {

	caCert, caKey, err := TryLoadCertAndKeyFromDisk(pkiPath, caName)
	if err != nil {
		return errors.Wrap(err, "failed load ca from disk")
	}

	cert, key, err := CreateSignedCertAndKey(certTempl, caCert, caKey)
	if err != nil {
		return errors.Wrapf(err, "create cert and key for failed: %s", name)
	}
	kubeconfig, err := CreateKubeConfigFromCertificate(caCert, cert, key, apiserver)
	if err != nil {
		return err
	}
	filename := path.Join(pkiPath, name)
	if err := clientcmd.WriteToFile(*kubeconfig, filename); err != nil {
		return errors.Wrapf(err, "error write kubeconfig file to disk: %s", name)
	}

	return nil
}

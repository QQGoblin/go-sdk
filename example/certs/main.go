package main

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/pkiutil"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"
	"math"
	"math/big"
	"net"
	"time"
)

const (
	DefaultServerCertCommonName = "ruijie"
	DefaultServerCertFile       = "tls/server/tls.crt"
	DefaultServerKeyFile        = "tls/server/tls.key"
)

func main() {
	if err := kubeCerts(); err != nil {
		klog.Fatalf(err.Error())
	}
	if err := serverCerts(); err != nil {
		klog.Fatalf(err.Error())
	}
}

func serverCerts() error {

	// 创建证书模板
	notBefore, _ := time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	notAfter, _ := time.Parse("2006-01-02 15:04:05", "2170-01-01 00:00:00")
	serial, _ := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))

	serverCertTempl := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: DefaultServerCertCommonName,
		},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
		},
		DNSNames: []string{
			"*",
		},
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// 读取根证书
	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk("tls", "ca")
	if err != nil {
		return err
	}

	// 通过根证书生成 ServerCert 和 ServerKey
	cert, key, err := pkiutil.CreateSignedCertAndKey(serverCertTempl, caCert, caKey)
	if err != nil {
		return err
	}

	// 写入证书文件
	if err := certutil.WriteCert(DefaultServerCertFile, pkiutil.EncodeCertPEM(cert)); err != nil {
		return err
	}

	// 写入Key文件
	encoded, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return err
	}
	if err := keyutil.WriteKey(DefaultServerKeyFile, encoded); err != nil {
		return err
	}
	return nil
}

func kubeCerts() error {
	// create all cert
	pkiPath := "tls"
	caName := "ca"
	svcIp := "10.28.1.1"
	ctrlIP := "192.168.1.1"
	nodeIp := "172.24.1.1"
	nodeName := "node1"

	// get all template
	allTemplate, err := pkiutil.GetCertificateTemplates(ctrlIP, nodeIp, svcIp, nodeName)
	if err != nil {
		return err
	}

	// create all tls for master node, we use ca.crt as etcd-ca.crt
	certMasterNeed := []string{
		pkiutil.APIServerCertAndKeyBaseName,
		pkiutil.APIServerKubeletClientCertAndKeyBaseName,
		pkiutil.APIServerEtcdClientCertAndKeyBaseName,
		pkiutil.FrontProxyClientCertAndKeyBaseName,
	}
	for _, certName := range certMasterNeed {
		if err := pkiutil.GenerateCertificateFiles(pkiPath, caName, certName, allTemplate[certName]); err != nil {
			return err
		}
	}

	// create sa
	if err := pkiutil.GenerateServiceAccountKeyAndPublicKeyFiles(pkiPath, x509.RSA); err != nil {
		return err
	}

	// create kubeconfig for master
	kubeconfigMasterNeed := []string{
		pkiutil.AdminKubeConfigBaseName,
		pkiutil.ControllerManagerKubeConfigBaseName,
		pkiutil.SchedulerKubeConfigBaseName,
	}
	localEp := fmt.Sprintf("https://%s:6433", nodeIp)
	for _, kubeconfigName := range kubeconfigMasterNeed {
		if err := pkiutil.GenerateKubeConfigFiles(pkiPath, caName, kubeconfigName, allTemplate[kubeconfigName], localEp); err != nil {
			return err
		}
	}
	// create kubeconfig for worker
	kubeconfigWorkerNeed := []string{
		pkiutil.KubeletKubeConfigBaseName,
		pkiutil.KubeProxyKubeConfigBaseName,
	}
	ctrlEp := fmt.Sprintf("https://%s:6433", ctrlIP)
	for _, kubeconfigName := range kubeconfigWorkerNeed {
		if err := pkiutil.GenerateKubeConfigFiles(pkiPath, caName, kubeconfigName, allTemplate[kubeconfigName], ctrlEp); err != nil {
			return err
		}
	}
	return err
}

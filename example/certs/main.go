package main

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
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
	DefaultServerCertFile       = "tls/etcd/server.crt"
	DefaultServerKeyFile        = "tls/etcd/server.key"
	DefaultPKIPath              = "tls"
	DefaultCAName               = "ca"
)

var (
	kubeServiceIP string
	kubeCtrlIP    string
	kubeNodeIP    string
	kubeNodename  string
)

func init() {
	flag.StringVar(&kubeServiceIP, "svc-address", "", "kube-apiserver 集群内网 ip 地址")
	flag.StringVar(&kubeCtrlIP, "controller-address", "", "kube-apiserver 浮动 IP 地址")
	flag.StringVar(&kubeNodeIP, "address", "", "节点管理网 IP 地址")
	flag.StringVar(&kubeNodename, "nodename", "", "节点主机名")

}

func main() {

	if err := kubeCerts(kubeCtrlIP, kubeNodeIP, kubeServiceIP, kubeNodename); err != nil {
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
	caCert, caKey, err := pkiutil.TryLoadCertAndKeyFromDisk(DefaultPKIPath, DefaultCAName)
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

func kubeCerts(ctrlIP, nodeIP, svcIP string, nodeName string) error {

	// get all template
	allTemplate, err := pkiutil.GetCertificateTemplates(ctrlIP, nodeIP, svcIP, nodeName)
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
		if err := pkiutil.GenerateCertificateFiles(DefaultPKIPath, DefaultCAName, certName, allTemplate[certName]); err != nil {
			return err
		}
	}

	// create sa
	if err := pkiutil.GenerateServiceAccountKeyAndPublicKeyFiles(DefaultPKIPath, x509.RSA); err != nil {
		return err
	}

	// create kubeconfig for master
	kubeconfigMasterNeed := []string{
		pkiutil.AdminKubeConfigBaseName,
		pkiutil.ControllerManagerKubeConfigBaseName,
		pkiutil.SchedulerKubeConfigBaseName,
	}
	localEp := fmt.Sprintf("https://%s:6443", nodeIP)
	for _, kubeconfigName := range kubeconfigMasterNeed {
		if err := pkiutil.GenerateKubeConfigFiles(DefaultPKIPath, DefaultCAName, kubeconfigName, allTemplate[kubeconfigName], localEp); err != nil {
			return err
		}
	}
	// create kubeconfig for worker
	kubeconfigWorkerNeed := []string{
		pkiutil.KubeletKubeConfigBaseName,
		pkiutil.KubeProxyKubeConfigBaseName,
	}
	ctrlEp := fmt.Sprintf("https://%s:6443", ctrlIP)
	for _, kubeconfigName := range kubeconfigWorkerNeed {
		if err := pkiutil.GenerateKubeConfigFiles(DefaultPKIPath, DefaultCAName, kubeconfigName, allTemplate[kubeconfigName], ctrlEp); err != nil {
			return err
		}
	}
	return err
}

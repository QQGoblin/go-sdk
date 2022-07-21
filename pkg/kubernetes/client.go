package kubernetes

import (
	"crypto"
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"github.com/QQGoblin/go-sdk/pkg/pkiutil"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	"math"
	"math/big"
	"time"
)

// NewClientFromBundle CACert 信息嵌入在代码中，通过 CACert 创建 Kubernetes 客户端
func NewClientFromBundle(masterURL string) (*kubernetes.Clientset, error) {
	config, err := buildBundleConfig(masterURL)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

func buildBundleConfig(apiserver string) (*restclient.Config, error) {

	caCert, caKey, err := loadDefaultBundleCACertAndKey()
	if err != nil {
		klog.Errorf("error load default cert and key: %s", err.Error())
		return nil, err
	}

	// 生成 admin.conf 证书
	notBefore, _ := time.Parse("2006-01-02 15:04:05", "1970-01-01 00:00:00")
	notAfter, _ := time.Parse("2006-01-02 15:04:05", "2170-01-01 00:00:00")
	serial, _ := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	certTempl := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "kubernetes-admin",
			Organization: []string{pkiutil.SystemPrivilegedGroup},
		},
		SerialNumber:          serial,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}
	cert, key, err := pkiutil.CreateSignedCertAndKey(certTempl, caCert, caKey)
	if err != nil {
		klog.Errorf("error create admin.conf cert: %s", err.Error())
		return nil, err
	}
	config, err := pkiutil.CreateKubeConfigFromCertificate(caCert, cert, key, apiserver)
	if err != nil {
		klog.Errorf("Error building kubernetes clientset from bundle cert: %s", err.Error())
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(*config, nil).ClientConfig()
}

func loadDefaultBundleCACertAndKey() (*x509.Certificate, crypto.Signer, error) {
	// 读取内置的证书文件
	certs, err := cert.ParseCertsPEM([]byte(caCert))
	if err != nil {
		klog.Errorf("error parse bundle cacert: %s", err)
		return nil, nil, err
	}
	caCert := certs[0]

	caKey, err := pkiutil.TryLoadKeyFromString(caKey)
	if err != nil {
		klog.Errorf("load key failed:%v", err)
		return nil, nil, err
	}
	return caCert, caKey, nil
}

package kubeutils

import (
	cryptorand "crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/pkiutil"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"math"
	"math/big"
	"path/filepath"
	"time"
)

var (
	DefaultCacheDir = filepath.Join(homedir.HomeDir(), ".kube", "cache")
)

// bundleClientHelper 实现了一种使用在特殊场景的 genericclioptions.RESTClientGetter，用户可以通过 apiserver 地址和根证书创建 k8s client
type bundleClientHelper struct {
	apiserver    string
	clientConfig clientcmd.ClientConfig
}

var _ genericclioptions.RESTClientGetter = &bundleClientHelper{}

func NewBundleClientHelper(apiserver, caCert, caKey string) (*bundleClientHelper, error) {

	apiserverURL := fmt.Sprintf("https://%s:%d", apiserver, DefaultKubeApiserverPort)

	clientConfig, err := BundleClientConfig(apiserverURL, caCert, caKey)
	if err != nil {
		return nil, err
	}

	return &bundleClientHelper{
		apiserver:    apiserver,
		clientConfig: clientConfig,
	}, nil
}

func (f *bundleClientHelper) RESTClientGetter() genericclioptions.RESTClientGetter {
	return f
}

func (f *bundleClientHelper) ToRESTConfig() (*rest.Config, error) {
	return f.clientConfig.ClientConfig()
}

func (f *bundleClientHelper) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return f.clientConfig
}

func (f *bundleClientHelper) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err

	}
	discoveryCacheDir := filepath.Join(DefaultCacheDir, "discovery", fmt.Sprintf("%s_%d", f.apiserver, DefaultKubeApiserverPort))
	httpCacheDir := filepath.Join(DefaultCacheDir, "http")
	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(10*time.Minute))
}

func (f *bundleClientHelper) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

// ToClientset 是原生 genericclioptions.RESTClientGetter 没有的接口
func (f *bundleClientHelper) ToClientset() (*kubernetes.Clientset, error) {

	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(restConfig)
}

// ToDynamicClient 是原生 genericclioptions.RESTClientGetter 没有的接口
func (f *bundleClientHelper) ToDynamicClient() (dynamic.Interface, error) {
	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(restConfig)
}

// ToUnstructuredClient 是原生 genericclioptions.RESTClientGetter 没有的接口
func (f *bundleClientHelper) ToUnstructuredClient(namespace, resources, version, group string) (dynamic.ResourceInterface, error) {

	dynamicCli, err := f.ToDynamicClient()
	if err != nil {
		return nil, err
	}

	clusterCli := dynamicCli.Resource(schema.GroupVersionResource{
		Resource: resources,
		Version:  version,
		Group:    group,
	})

	if namespace != "" {
		return clusterCli.Namespace(namespace), nil
	}
	return clusterCli, nil
}

// BundleClientConfig create a new admin.conf for connect apiserver
func BundleClientConfig(apiserverURL, caCertStr, caKeyStr string) (clientcmd.ClientConfig, error) {

	caCert, caKey, err := pkiutil.LoadCertificateAndKeyFromString(caCertStr, caKeyStr)
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
	config, err := pkiutil.CreateKubeConfigFromCertificate(caCert, cert, key, apiserverURL)
	if err != nil {
		klog.Errorf("Error building kubernetes clientset from bundle cert: %s", err.Error())
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(*config, nil), nil
}

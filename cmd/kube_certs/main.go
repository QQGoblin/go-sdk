package main

import (
	"crypto/x509"
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/pkiutil"
	"k8s.io/klog/v2"
)

func main() {
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
		klog.Fatal(err)
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
			klog.Fatal(err)
		}
	}

	// create sa
	if err := pkiutil.GenerateServiceAccountKeyAndPublicKeyFiles(pkiPath, x509.RSA); err != nil {
		klog.Fatal(err)
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
			klog.Fatal(err)
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
			klog.Fatal(err)
		}
	}
}

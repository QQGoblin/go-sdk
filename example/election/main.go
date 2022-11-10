package main

import (
	"context"
	"encoding/json"
	"fmt"
	leaderelection "github.com/QQGoblin/go-sdk/pkg/leaderelection"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {

	kubeconfig := "/etc/kubernetes/admin.conf"
	if len(os.Args) > 1 {
		kubeconfig = os.Args[1]
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	kubeCli, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Error create kubernetes clientset: %s", err.Error())
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Error get node hostname: %s", err.Error())
	}

	rlc := resourcelock.ResourceLockConfig{
		Identity:      id,
		EventRecorder: nil, // 暂时不添加
	}

	lock, err := resourcelock.New(resourcelock.LeasesResourceLock, "default", "hello", kubeCli.CoreV1(), kubeCli.CoordinationV1(), rlc)
	if err != nil {
		klog.Fatalf("error create lease resource lock: %s", err.Error())
	}

	leaderElectionConfig := leaderelection.LeaderElectionConfig{
		Lock:          lock,
		LeaseDuration: time.Second * 15,
		RenewDeadline: time.Second * 10,
		RetryPeriod:   time.Second * 3,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("I'm new leader, my name is %s", id)
			},
			OnStoppedLeading: func() {
				klog.Info("I'm no leader anymore")
			},
		},
		ReleaseOnCancel: true, // 当 context 退出时，释放 lock
	}

	dockerSimpleClient, err := dockerSimpleClient(DefaultDockerSocket, DefaultTimeOut)
	if err != nil {
		klog.Fatalf("error create docker client: %+v", err.Error())
	}

	LeaderHealthCheck := func(context.Context) error {
		return dockerIsRunning(dockerSimpleClient, DefaultDockerSocket)
	}

	lewc, err := leaderelection.NewLeaderElectorWithConditions(id, leaderElectionConfig, LeaderHealthCheck)
	if err != nil {
		klog.Fatalf("error create leader elector: %+v", err.Error())
	}

	lewc.Startup()

	stopCh := make(chan os.Signal, 0)
	signal.Notify(stopCh, os.Interrupt, os.Kill)
	<-stopCh
	lewc.Stop()
	klog.Info("exit success.")

}

const (
	DefaultDockerSocket = "/var/run/docker.sock"
	DefaultTimeOut      = 1 * time.Second
)

// 检查 docker daemon 是否在运行，参考：curl -XGET --unix-socket /var/run/docker.sock  -H 'Content-Type: application/json' http://localhost/info

type DockerInfo struct {
	ID                string `json:"ID"`
	Containers        int    `json:"Containers"`
	ContainersRunning int    `json:"ContainersRunning"`
	ContainersPaused  int    `json:"ContainersPaused"`
	ContainersStopped int    `json:"ContainersStopped"`
	ServerVersion     string `json:"ServerVersion"`
}

func dockerSimpleClient(socket string, timeout time.Duration) (*http.Client, error) {
	tr := new(http.Transport)
	tr.DisableCompression = true
	tr.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.DialTimeout("unix", socket, timeout)
	}

	client := &http.Client{Transport: tr}

	if _, ok := client.Transport.(http.RoundTripper); !ok {
		return nil, fmt.Errorf("unable to verify tls configuration, invalid transport: %v", client.Transport)
	}
	return client, nil
}

func dockerIsRunning(cli *http.Client, dockerSocket string) error {

	req, err := http.NewRequest("GET", "/info", nil)
	if err != nil {
		return fmt.Errorf("error creat request")
	}
	req.URL.Scheme = "http"
	req.URL.Host = dockerSocket

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)

	info := &DockerInfo{}
	if err := json.Unmarshal(b, info); err != nil {
		return fmt.Errorf("unable unmarshal resp")
	}
	klog.Infof("docker-daemon health check succeeded. The information is as follows: %+v", info)

	return nil
}

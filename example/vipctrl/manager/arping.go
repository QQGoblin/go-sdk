package manager

import (
	"github.com/QQGoblin/go-sdk/pkg/network"
	"k8s.io/klog/v2"
	"sync"
	"time"
)

type arpingHelper struct {
	address  string
	bindLink string
	ticker   *time.Ticker
	stopChan chan struct{}
	mutex    sync.Mutex
}

func NewArpingHelper(address, bindlink string) *arpingHelper {

	ticker := time.NewTicker(1100 * time.Millisecond)
	ticker.Stop()
	return &arpingHelper{
		address:  address,
		bindLink: bindlink,
		ticker:   ticker,
	}
}

// Start 开始 arping 广播要求接口可重入
func (r *arpingHelper) Start() {

	// stopChan 非空时直接返回，保证可重入
	if r.stopChan != nil {
		return
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.stopChan != nil {
		r.stopChan <- struct{}{}
		// 不显示 close chan ，等待系统自动回收
		// close(r.stopChan)
	}

	r.stopChan = make(chan struct{}, 1)

	go r.arping()
}

// Stop 停止 arping 广播，要求接口可重入
func (r *arpingHelper) Stop() {

	// start 和 stop 添加互斥
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.stopChan != nil {
		klog.Info("stop sending ARPING broadcast.")
		r.stopChan <- struct{}{}
		close(r.stopChan)
		r.stopChan = nil
	}

}

func (r *arpingHelper) arping() {

	r.ticker.Reset(1100 * time.Millisecond)
	klog.Info("start sending ARPING broadcast.")
	for {
		select {
		case <-r.ticker.C:
			network.ARPSendGratuitous(r.address, r.bindLink)
		case <-r.stopChan:
			r.ticker.Stop()
			goto END

		}
	}
END:
}

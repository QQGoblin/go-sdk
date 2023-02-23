package manager

import (
	"fmt"
	"github.com/QQGoblin/go-sdk/pkg/network"
	"github.com/QQGoblin/go-sdk/pkg/sysctl"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"sync"
)

type macvlanHelper struct {
	bindLink    string
	mac         string
	address     *netlink.Addr
	macvlanLink string
	mutex       sync.Mutex
}

func NewMacvlanHelper(ip, mask string, bindLink, macvlanLink string, mac string) (*macvlanHelper, error) {

	vipMask, err := network.ParseIPV4Mask(mask)

	if err != nil {
		return nil, err
	}

	netSize, _ := vipMask.Size()

	address, err := netlink.ParseAddr(fmt.Sprintf("%s/%d", ip, netSize))

	if err != nil {
		return nil, err
	}

	return &macvlanHelper{
		bindLink:    bindLink,
		macvlanLink: macvlanLink,
		mac:         mac,
		address:     address,
	}, nil
}

// Set 创建 macvlan 设备
func (o *macvlanHelper) Set() error {

	// 判断 macvlan 设备是否已经创建
	if o.isExist() {
		return nil
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	// 创建 macvlan 设备

	klog.Info("create the macvlan device")

	mDev, err := network.CreateMacvlanDevice(o.macvlanLink, o.bindLink, o.mac)
	if err != nil {
		return err
	}

	// 设置 ip 地址
	err = netlink.AddrAdd(mDev, o.address)
	if err != nil {
		return err
	}

	// macvlan arping 指标响应
	_, _ = sysctl.Sysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_notify", o.macvlanLink), "1")

	// 启动网络设备
	klog.Info("set macvlan device up")
	if err := netlink.LinkSetUp(mDev); err != nil {
		return err
	}

	return nil
}

func (o *macvlanHelper) Delete() error {

	o.mutex.Lock()
	defer o.mutex.Unlock()

	l, err := netlink.LinkByName(o.macvlanLink)
	if err != nil {
		_, ok := err.(netlink.LinkNotFoundError)
		if ok {
			return nil
		}
		return errors.Wrapf(err, "mgmt_device<%s> link not found: %+v", o.macvlanLink, err)
	}

	klog.Info("remove the macvlan device")

	return netlink.LinkDel(l)
}

func (o *macvlanHelper) isExist() bool {

	l, err := netlink.LinkByName(o.macvlanLink)
	if err != nil {
		return false
	}

	addrList, err := netlink.AddrList(l, 0)
	if err != nil {
		return false
	}
	for _, addr := range addrList {
		if o.address.Equal(addr) {
			return true
		}
	}
	return false
}

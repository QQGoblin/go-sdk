package network

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"net"
)

func CreateMacvlanDevice(name, master, mac string) (netlink.Link, error) {

	m, err := netlink.LinkByName(master)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup master %q: %v", master, err)
	}

	linkAttrs := netlink.LinkAttrs{
		Name:        name,
		ParentIndex: m.Attrs().Index,
	}

	if mac != "" {
		addr, err := net.ParseMAC(mac)
		if err != nil {
			return nil, fmt.Errorf("invalid args %v for MAC addr: %v", mac, err)
		}
		linkAttrs.HardwareAddr = addr
	}

	mv := &netlink.Macvlan{
		LinkAttrs: linkAttrs,
		Mode:      netlink.MACVLAN_MODE_BRIDGE,
	}

	if err := netlink.LinkAdd(mv); err != nil {
		return nil, fmt.Errorf("failed to create macvlan: %v", err)
	}

	macvlan, err := netlink.LinkByName(name)
	if err != nil {
		return nil, errors.Wrapf(err, "macvlan<%s> link not found: %v", name, err)
	}

	return macvlan, nil
}

package network

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Get local Address from local route table.
// Copy from kube-proxy src.
func GetLocalAddresses(dev string, filterDevs ...string) (sets.String, error) {

	h := netlink.Handle{}
	chosenLinkIndex := -1
	if dev != "" {
		link, err := h.LinkByName(dev)
		if err != nil {
			return nil, fmt.Errorf("error get device %s, err: %v", dev, err)
		}
		chosenLinkIndex = link.Attrs().Index
	}
	filterLinkIndexSet := sets.NewInt()
	for _, filterDev := range filterDevs {
		link, err := h.LinkByName(filterDev)
		if err != nil {
			return nil, fmt.Errorf("error get filter device %s, err: %v", filterDev, err)
		}
		filterLinkIndexSet.Insert(link.Attrs().Index)
	}

	routeFilter := &netlink.Route{
		Table:    unix.RT_TABLE_LOCAL,
		Type:     unix.RTN_LOCAL,
		Protocol: unix.RTPROT_KERNEL,
	}
	filterMask := netlink.RT_FILTER_TABLE | netlink.RT_FILTER_TYPE | netlink.RT_FILTER_PROTOCOL

	// find chosen device
	if chosenLinkIndex != -1 {
		routeFilter.LinkIndex = chosenLinkIndex
		filterMask |= netlink.RT_FILTER_OIF
	}

	routes, err := h.RouteListFiltered(netlink.FAMILY_ALL, routeFilter, filterMask)
	if err != nil {
		return nil, fmt.Errorf("error list route table, err: %v", err)
	}
	res := sets.NewString()
	for _, route := range routes {
		if filterLinkIndexSet.Has(route.LinkIndex) {
			continue
		}
		if route.Src != nil {
			res.Insert(route.Src.String())
		}
	}
	return res, nil
}

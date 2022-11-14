package network

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

const (
	opARPRequest = 1
	opARPReply   = 2
	hwLen        = 6
)

var (
	ethernetBroadcast = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	// arpRequest is used to flip between garp request or garp reply
	arpRequest = true
)

func htons(p uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], p)
	return *(*uint16)(unsafe.Pointer(&b))
}

// arpHeader specifies the header for an ARP message.
type arpHeader struct {
	hardwareType          uint16
	protocolType          uint16
	hardwareAddressLength uint8
	protocolAddressLength uint8
	opcode                uint16
}

// arpMessage represents an ARP message.
type arpMessage struct {
	arpHeader
	senderHardwareAddress []byte
	senderProtocolAddress []byte
	targetHardwareAddress []byte
	targetProtocolAddress []byte
}

// bytes returns the wire representation of the ARP message.
func (m *arpMessage) bytes() ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, m.arpHeader); err != nil {
		return nil, fmt.Errorf("binary write failed: %v", err)
	}
	buf.Write(m.senderHardwareAddress)
	buf.Write(m.senderProtocolAddress)
	buf.Write(m.targetHardwareAddress)
	buf.Write(m.targetProtocolAddress)

	return buf.Bytes(), nil
}

// gratuitousARP return a gARP request or gARP reply alternatively
// because different devices may support either one of them
func gratuitousARP(ip net.IP, mac net.HardwareAddr) (*arpMessage, error) {
	if ip.To4() == nil {
		return nil, fmt.Errorf("%q is not an IPv4 address", ip)
	}
	if len(mac) != hwLen {
		return nil, fmt.Errorf("%q is not an Ethernet MAC address", mac)
	}

	m := &arpMessage{
		arpHeader: arpHeader{
			1,           // Ethernet
			0x0800,      // IPv4
			hwLen,       // 48-bit MAC Address
			net.IPv4len, // 32-bit IPv4 Address
			opARPReply,  // ARP Reply
		},
	}

	// https://tools.ietf.org/html/rfc5944#section-4.6
	// In either case, the ARP Sender Hardware Address is
	// set to the link-layer address to which this cache entry should be
	// updated.
	m.senderHardwareAddress = mac

	// When using an ARP Reply packet, the Target Hardware
	// Address is also set to the link-layer address to which this cache
	// entry should be updated (this field is not used in an ARP Request
	// packet).
	m.targetHardwareAddress = mac

	// In either case, the ARP Sender Protocol Address and
	// ARP Target Protocol Address are both set to the IP address of the
	// cache entry to be updated,
	m.senderProtocolAddress = ip.To4()
	m.targetProtocolAddress = ip.To4()

	// send arpRequest and arpReply alternatively
	// 参考：https://www.practicalnetworking.net/series/arp/gratuitous-arp/ 和 https://www.practicalnetworking.net/series/arp/arp-probe-arp-announcement/
	// Gratutious 和 Announcement 格式的 ARP 包均能对外宣告自己的 mac 信息，两者只是包头的 opcode 存在差异
	// 此处连续调用 gratuitousARP 方法时，会交替生成这两种格式 arp 包
	arpRequest = !arpRequest
	if arpRequest {
		m.arpHeader.opcode = opARPRequest

		// this field is not used in an ARP Request packet
		m.targetHardwareAddress = ethernetBroadcast
	}

	return m, nil
}

// sendARP sends the given ARP message via the specified interface.
func sendARP(iface *net.Interface, m *arpMessage) error {
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_DGRAM, int(htons(syscall.ETH_P_ARP)))
	if err != nil {
		return fmt.Errorf("failed to get raw socket: %v", err)
	}
	defer syscall.Close(fd)

	if err := syscall.BindToDevice(fd, iface.Name); err != nil {
		return fmt.Errorf("failed to bind to device: %v", err)
	}

	ll := syscall.SockaddrLinklayer{
		Protocol: htons(syscall.ETH_P_ARP),
		Ifindex:  iface.Index,
		Pkttype:  0, // syscall.PACKET_HOST
		Hatype:   m.hardwareType,
		Halen:    m.hardwareAddressLength,
	}
	target := ethernetBroadcast
	for i := 0; i < len(target); i++ { //nolint
		ll.Addr[i] = target[i]
	}

	b, err := m.bytes()
	if err != nil {
		return fmt.Errorf("failed to convert ARP message: %v", err)
	}

	if err := syscall.Bind(fd, &ll); err != nil {
		return fmt.Errorf("failed to bind: %v", err)
	}
	if err := syscall.Sendto(fd, b, 0, &ll); err != nil {
		return fmt.Errorf("failed to send: %v", err)
	}

	return nil
}

// ARPSendGratuitous sends a gratuitous ARP message via the specified interface.
func ARPSendGratuitous(address, ifaceName string, count int) error {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return fmt.Errorf("failed to get interface %q: %v", ifaceName, err)
	}

	ip := net.ParseIP(address)
	if ip == nil {
		return fmt.Errorf("failed to parse address %s", ip)
	}

	for i := 0; i < count; i++ {
		m, err := gratuitousARP(ip, iface.HardwareAddr)

		if err != nil {
			return err
		}

		if err := sendARP(iface, m); err != nil {
			return err
		}
		// This is a debug message, enable debugging to ensure that the gratuitous arp is repeating
		fmt.Sprintf("Broadcasting ARP update for %s (%s) via %s", address, iface.HardwareAddr, iface.Name)
	}

	return nil
}
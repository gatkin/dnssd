package dnssd

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

const (
	ipv4UDPNetwork = "udp4"
	ipv6UDPNetwork = "udp6"
)

const (
	// From RFC 6762
	mdnsPort   = 5353
	mdnsIPv4IP = "224.0.0.251"
	mdnsIPv6IP = "FF02::FB"
)

var (
	mdnsIPv4Addr = net.UDPAddr{
		IP:   net.ParseIP(mdnsIPv4IP),
		Port: mdnsPort,
	}

	mdnsIPv6Addr = net.UDPAddr{
		IP:   net.ParseIP(mdnsIPv6IP),
		Port: mdnsPort,
	}
)

// netClient provides access to sending and receiving network messages.
type netClient struct {
	msgCh          <-chan dns.Msg
	multicastConns []udpConnection
	unicastConns   []udpConnection
}

// udpConnection represents a single UDP connection.
type udpConnection struct {
	conn       *net.UDPConn
	shutdownCh chan struct{}
}

// newNetClient creates a new network client listening for DNS messages on the specified interfaces
// and address families.
func newNetClient(addrFamily AddrFamily, interfaces []net.Interface) (client netClient, err error) {
	var unicastConns, multicastConns []udpConnection
	msgCh := make(chan dns.Msg)

	unicastConns, err = unicastConnectionsCreate(addrFamily, msgCh)
	if err != nil {
		return
	}

	multicastConns, err = multicastConnectionsCreate(addrFamily, interfaces, msgCh)
	if err != nil {
		return
	}

	client = netClient{
		msgCh:          msgCh,
		multicastConns: multicastConns,
		unicastConns:   unicastConns,
	}

	return
}

// multicastConnectionsCreate creates all multicast connections.
func multicastConnectionsCreate(addrFamily AddrFamily, interfaces []net.Interface, msgCh chan<- dns.Msg) (conns []udpConnection, err error) {
	conns = make([]udpConnection, 0)

	for _, ifi := range interfaces {
		var conn udpConnection

		if addrFamily.includesIPv4() {
			conn, err = newMulticastConnection(ipv4UDPNetwork, &ifi, msgCh)
			if err != nil {
				return
			}

			conns = append(conns, conn)
		}

		if addrFamily.includesIPv6() {
			conn, err = newMulticastConnection(ipv6UDPNetwork, &ifi, msgCh)
			if err != nil {
				return
			}

			conns = append(conns, conn)
		}
	}

	return
}

// newMulticastConnection creates a new multicast connection on the given network and interface.
// all received messages will be sent to the provided message channel.
func newMulticastConnection(network string, ifi *net.Interface, msgCh chan<- dns.Msg) (conn udpConnection, err error) {
	conn = udpConnection{
		shutdownCh: make(chan struct{}),
	}

	var groupAddr *net.UDPAddr
	if network == ipv4UDPNetwork {
		groupAddr = &mdnsIPv4Addr
	} else {
		groupAddr = &mdnsIPv6Addr
	}

	conn.conn, err = net.ListenMulticastUDP(network, ifi, groupAddr)
	if err != nil {
		err = fmt.Errorf("dnssd: failed creating multicast connection on network %v interface %v: %v", network, ifi, err)
		return
	}

	go conn.listen(msgCh)

	return
}

// newUnicastConnection creates a new unicast UDP connection on the specified network. All
// received messages will be written to the given channel.
func newUnicastConnection(network string, msgCh chan<- dns.Msg) (conn udpConnection, err error) {
	conn = udpConnection{
		shutdownCh: make(chan struct{}),
	}

	conn.conn, err = net.ListenUDP(network, &net.UDPAddr{})
	if err != nil {
		err = fmt.Errorf("dnssd: failed to create unicast connection on network %v: %v", network, err)
		return
	}

	go conn.listen(msgCh)

	return
}

// unicastConnectionsCreate creates all unicast connections.
func unicastConnectionsCreate(addrFamily AddrFamily, msgCh chan<- dns.Msg) (conns []udpConnection, err error) {
	conns = make([]udpConnection, 0)

	var conn udpConnection

	if addrFamily.includesIPv4() {
		conn, err = newUnicastConnection(ipv4UDPNetwork, msgCh)
		if err != nil {
			return
		}

		conns = append(conns, conn)
	}

	if addrFamily.includesIPv6() {
		conn, err = newUnicastConnection(ipv6UDPNetwork, msgCh)
		if err != nil {
			return
		}

		conns = append(conns, conn)
	}

	return
}

// includesIPv4 returns true if the address family includes IPv4 support.
func (a AddrFamily) includesIPv4() bool {
	return (a == IPv4) || (a == AllAddrFamilies)
}

// includesIPv6 returns true if the address family includes IPv6 support.
func (a AddrFamily) includesIPv6() bool {
	return (a == IPv6) || (a == AllAddrFamilies)
}

// close closes the network client.
func (c *netClient) close() {
	for _, conn := range c.multicastConns {
		conn.close()
	}

	for _, conn := range c.unicastConns {
		conn.close()
	}
}

// close closes the connection.
func (c *udpConnection) close() {
	go func() {
		c.shutdownCh <- struct{}{}
	}()
}

// listen listens for DNS messages on the UDP connection writing received messages to the
// provided channel.
func (c *udpConnection) listen(msgCh chan<- dns.Msg) {
	const (
		maxPacketSize = 9000 // Defined in RFC 6762 Section 17
	)

	defer c.conn.Close()

	readBuf := make([]byte, maxPacketSize)
	for {
		bytesRead, err := c.conn.Read(readBuf)

		// Check to see if we have been told to shutdown while we were waiting
		select {
		case <-c.shutdownCh:
			return
		default:
		}

		if err != nil {
			log.Printf("dnssd: failed to read from UDP connection: %v", err)
			continue
		}

		msg := dns.Msg{}
		err = msg.Unpack(readBuf[:bytesRead])
		if err != nil {
			log.Printf("dnssd: failed parsing DNS packet: %v", err)
			continue
		}

		msgCh <- msg
	}
}

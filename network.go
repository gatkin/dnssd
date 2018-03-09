package dnssd

import (
	"fmt"
	"log"
	"net"

	"github.com/miekg/dns"
)

type udpNetwork string

const (
	ipv4UDPNetwork = udpNetwork("udp4")
	ipv6UDPNetwork = udpNetwork("udp6")
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
	multicastConns []udpConnection
	unicastConns   []udpConnection
}

// udpConnection represents a single UDP connection.
type udpConnection struct {
	conn       *net.UDPConn
	network    udpNetwork
	shutdownCh chan struct{}
}

// question represents a DNS question to be sent on the network.
type question interface {
	toDNSQuestion() dns.Question
}

// pointerQuestion represents a PTR DNS question to be set.
type pointerQuestion struct {
	serviceName string
}

// interfaceGetAddresses returns all IP addresses for the given interface.
func interfaceGetAddresses(ifi net.Interface) ([]net.IP, error) {
	interfaceIPs := make([]net.IP, 0)

	addrs, err := ifi.Addrs()
	if err != nil {
		return interfaceIPs, err
	}

	for _, addr := range addrs {
		switch ipNet := addr.(type) {
		case *net.IPNet:
			interfaceIPs = append(interfaceIPs, ipNet.IP)
		}
	}

	return interfaceIPs, err
}

// newNetClient creates a new network client listening for DNS messages on the specified interfaces
// and address families.
func newNetClient(addrFamily AddrFamily, interfaces []net.Interface, msgCh chan<- dns.Msg) (client netClient, err error) {
	var unicastConns, multicastConns []udpConnection

	unicastConns, err = unicastConnectionsCreate(addrFamily, interfaces, msgCh)
	if err != nil {
		return
	}

	multicastConns, err = multicastConnectionsCreate(addrFamily, interfaces, msgCh)
	if err != nil {
		return
	}

	client = netClient{
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
func newMulticastConnection(network udpNetwork, ifi *net.Interface, msgCh chan<- dns.Msg) (conn udpConnection, err error) {
	conn = udpConnection{
		network:    network,
		shutdownCh: make(chan struct{}),
	}

	var groupAddr *net.UDPAddr
	if network == ipv4UDPNetwork {
		groupAddr = &mdnsIPv4Addr
	} else {
		groupAddr = &mdnsIPv6Addr
	}

	conn.conn, err = net.ListenMulticastUDP(string(network), ifi, groupAddr)
	if err != nil {
		err = fmt.Errorf("dnssd: failed creating multicast connection on network %v interface %v: %v", network, ifi, err)
		return
	}

	go conn.listen(msgCh)

	return
}

// newUnicastConnection creates a new unicast UDP connection on the specified network. All
// received messages will be written to the given channel.
func newUnicastConnection(network udpNetwork, interfaceIP net.IP, msgCh chan<- dns.Msg) (conn udpConnection, err error) {
	conn = udpConnection{
		network:    network,
		shutdownCh: make(chan struct{}),
	}

	conn.conn, err = net.ListenUDP(string(network), &net.UDPAddr{IP: interfaceIP})
	if err != nil {
		err = fmt.Errorf("dnssd: failed to create unicast connection on network %v: %v", network, err)
		return
	}

	go conn.listen(msgCh)

	return
}

// unicastConnectionsCreate creates all unicast connections.
func unicastConnectionsCreate(addrFamily AddrFamily, interfaces []net.Interface, msgCh chan<- dns.Msg) ([]udpConnection, error) {
	conns := make([]udpConnection, 0)

	for _, ifi := range interfaces {
		ipAddrs, err := interfaceGetAddresses(ifi)
		if err != nil {
			return conns, err
		}

		for _, addr := range ipAddrs {
			if addrFamily.includesIPv4() && addr.To4() != nil {
				conn, err := newUnicastConnection(ipv4UDPNetwork, addr, msgCh)
				if err != nil {
					return conns, err
				}

				conns = append(conns, conn)
			} else if addrFamily.includesIPv6() {
				conn, err := newUnicastConnection(ipv6UDPNetwork, addr, msgCh)
				if err != nil {
					return conns, err
				}

				conns = append(conns, conn)
			}
		}
	}

	return conns, nil
}

// includesIPv4 returns true if the address family includes IPv4 support.
func (a AddrFamily) includesIPv4() bool {
	return (a == AddrFamilyIPv4) || (a == AddrFamilyAll)
}

// includesIPv6 returns true if the address family includes IPv6 support.
func (a AddrFamily) includesIPv6() bool {
	return (a == AddrFamilyIPv6) || (a == AddrFamilyAll)
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

// sendQuestion sends the given question.
func (c *netClient) sendQuestion(q question) error {
	return c.sendQuestions([]question{q})
}

// sendQuestions sends the given set questions.
func (c *netClient) sendQuestions(questions []question) error {
	dnsQuestions := make([]dns.Question, 0, len(questions))
	for i := range questions {
		dnsQuestions = append(dnsQuestions, questions[i].toDNSQuestion())
	}

	message := dns.Msg{
		Question: dnsQuestions,
	}

	data, err := message.Pack()
	if err != nil {
		return err
	}

	for _, conn := range c.unicastConns {
		var groupAddr *net.UDPAddr
		if conn.network == ipv4UDPNetwork {
			groupAddr = &mdnsIPv4Addr
		} else {
			groupAddr = &mdnsIPv6Addr
		}

		_, err := conn.conn.WriteToUDP(data, groupAddr)
		if err != nil {
			return err
		}
	}

	return nil
}

// close closes the connection.
func (c *udpConnection) close() {
	go func() {
		c.conn.Close()
		c.shutdownCh <- struct{}{}
	}()
}

// listen listens for DNS messages on the UDP connection writing received messages to the
// provided channel.
func (c *udpConnection) listen(msgCh chan<- dns.Msg) {
	const (
		maxPacketSize = 9000 // Defined in RFC 6762 Section 17
	)

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

// toDNSQuestion converts the pointer question into the corresponding DNS question.
func (p pointerQuestion) toDNSQuestion() dns.Question {
	return dns.Question{
		Name:   p.serviceName,
		Qtype:  dns.TypePTR,
		Qclass: dns.ClassINET,
	}
}

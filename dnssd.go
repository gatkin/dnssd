// Package dnssd discovers services on a local area network over mDNS.
package dnssd

import (
	"net"

	"github.com/miekg/dns"
)

// AddrFamily represents an address family on which to browse for services.
type AddrFamily int

// Indicates the address families on which to browse for services.
const (
	IPv4 AddrFamily = iota
	IPv6
	AllAddrFamilies
)

// Resolver browses for services on a local area network advertised via mDNS.
type Resolver struct {
	messagePipeline messagePipeline
	netClient       netClient
	shutdownCh      chan struct{}
}

// NewResolver creates a new resolver listening for mDNS messages on the specified interfaces.
func NewResolver(addrFamily AddrFamily, interfaces []net.Interface) (resolver Resolver, err error) {
	msgCh := make(chan dns.Msg)

	var client netClient
	client, err = newNetClient(addrFamily, interfaces, msgCh)
	if err != nil {
		return
	}

	messagePipeline := newMessagePipeline()
	go messagePipeline.pipeMessages(msgCh)

	resolver = Resolver{
		messagePipeline: messagePipeline,
		netClient:       client,
		shutdownCh:      make(chan struct{}),
	}

	go resolver.browse()

	return
}

// Close closes the resolver and cleans up all resources owned by it.
func (r *Resolver) Close() {
	go func() {
		r.shutdownCh <- struct{}{}
	}()
}

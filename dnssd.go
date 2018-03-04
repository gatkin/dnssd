// Package dnssd discovers services on a local area network over DNS.
package dnssd

import (
	"fmt"
	"net"
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
	netClient netClient
}

// NewResolver creates a new resolver listening for mDNS messages on the specified interfaces.
func NewResolver(addrFamily AddrFamily, interfaces []net.Interface) (resolver Resolver, err error) {
	var client netClient
	client, err = newNetClient(addrFamily, interfaces)
	if err != nil {
		return
	}

	resolver = Resolver{
		netClient: client,
	}

	go resolver.browse()

	return
}

// Close closes the resolver and cleans up all resources owned by it.
func (r *Resolver) Close() {
	r.netClient.close()
}

func (r *Resolver) browse() {
	for msg := range r.netClient.msgCh {
		fmt.Printf("%v\n", msg)
	}
}

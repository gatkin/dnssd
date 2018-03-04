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
	cache             cache
	messagePipeline   messagePipeline
	netClient         netClient
	resolvedInstances map[serviceInstanceID]ServiceInstance
	serviceAddCh      chan string
	services          map[string]bool // Set of services being browsed for
	shutdownCh        chan struct{}
}

// ServiceInstance represents a discovered instance of a service.
type ServiceInstance struct {
	Address      net.IP
	InstanceName string
	Port         uint16
	ServiceName  string
	TextRecords  map[string]string
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

	resolver = Resolver{
		cache:             newCache(),
		messagePipeline:   messagePipeline,
		netClient:         client,
		resolvedInstances: make(map[serviceInstanceID]ServiceInstance),
		serviceAddCh:      make(chan string),
		services:          make(map[string]bool),
		shutdownCh:        make(chan struct{}),
	}

	go messagePipeline.pipeMessages(msgCh)
	go resolver.browse()

	return
}

// AddService adds the given service to the set of services the resolver is browsing for. This has
// no effect if the resolver is already browsing for the service.
func (r *Resolver) AddService(name string) {
	r.serviceAddCh <- name
}

// Close closes the resolver and cleans up all resources owned by it.
func (r *Resolver) Close() {
	go func() {
		r.shutdownCh <- struct{}{}
	}()
}

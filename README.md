# DNS-Based Service Discovery for Go

[![Go Doc](https://godoc.org/github.com/gatkin/dnssd?status.svg)](http://godoc.org/github.com/gatkin/dnssd)
[![Build Status](https://travis-ci.org/gatkin/dnssd.svg?branch=master)](https://travis-ci.org/gatkin/dnssd)
[![Go Report Card](https://goreportcard.com/badge/github.com/gatkin/dnssd)](https://goreportcard.com/report/github.com/gatkin/dnssd)

DNS-Based service discovery, or DNS-SD ([RFC 6763](https://tools.ietf.org/html/rfc6763)), can be used to discover services on a local area network advertised over Multicast DNS, or mDNS ([RFC 6762](https://tools.ietf.org/html/rfc6762)), enabling peer-to-peer discovery.

This library provides DNS-SD capabilities in Go. To use it, you first create a resolver by specifying the interfaces and address family on which to browse for services.

```go
ifi, err := net.InterfaceByName("en0")
if err != nil {
    log.Fatal(err)
}

resolver, err := dnssd.NewResolver(dnssd.AddrFamilyIPv4, []net.Interface{*ifi})
if err != nil {
    log.Fatal(err)
}
defer resolver.Close()
```

Next, you provide the names of the services which you wish to browse for to the resolver. The same resolver can be used to browse for multiple services.

```go
resolver.BrowseService("_http._tcp.local.")
resolver.BrowseService("_googlecast._tcp.local.")
```

Finally, you can query the resolver for the set of fully resolved service instances by either retrieving all resolved instances or only instances for a particular service.

```go
instances := resolver.GetAllResolvedInstances()
for _, instance := range instances {
    fmt.Printf("%v\n", instance)
}

chromecasts := resolver.GetResolvedInstances("_googlecast._tcp.local.")
for _, instance := range instances {
    fmt.Printf("%v\n", instance)
}
```

We can put all of this together to discover all instances of the `_http._tcp` service on the local network

```go
package main

import (
    "fmt"
    "log"
    "net"
    "time"

    "github.com/gatkin/dnssd"
)

func main() {
    ifi, err := net.InterfaceByName("en0")
    if err != nil {
        log.Fatal(err)
    }

    resolver, err := dnssd.NewResolver(dnssd.AddrFamilyIPv4, []net.Interface{*ifi})
    if err != nil {
        log.Fatal(err)
    }
    defer resolver.Close()

    resolver.BrowseService("_http._tcp.local.")
    resolver.BrowseService("_googlecast._tcp.local.")

    // Wait some time to allow all service instances to be discovered.
    time.Sleep(1 * time.Second)

    instances := resolver.GetAllResolvedInstances()
    for _, instance := range instances {
        fmt.Printf("%v\n", instance)
    }
}
```
package dnssd

import (
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// messagePipeline filters, transforms, and pipes raw DNS messages to the appropriate
// output channels.
type messagePipeline struct {
	addressRecordCh chan addressRecord
	pointerRecordCh chan pointerRecord
	serviceRecordCh chan serviceRecord
	shutdownCh      chan struct{}
	textRecordCh    chan textRecord
}

// addressRecord contains received address information.
type addressRecord struct {
	address net.IP
	name    string
	resourceRecord
}

// pointerRecord contains information received for an instance's PTR record.
type pointerRecord struct {
	instanceName string
	serviceName  string
	resourceRecord
}

// resourceRecord contains fields common to all resource records.
type resourceRecord struct {
	cacheFlush bool
	timeToLive time.Duration
}

// serviceRecord contains information received for an instance's SRV record.
type serviceRecord struct {
	instanceName string
	port         uint16
	serviceName  string
	target       string
	resourceRecord
}

// textRecord contains information received for an instances TXT record.
type textRecord struct {
	instanceName string
	serviceName  string
	values       map[string]string
	resourceRecord
}

// aaaaToAddressRecord converts an AAAA record into an address record
func aaaaToAddressRecord(aaaa *dns.AAAA) addressRecord {
	return addressRecord{
		address:        aaaa.AAAA,
		name:           aaaa.Hdr.Name,
		resourceRecord: headerToResourceRecord(&aaaa.Hdr),
	}
}

// aToAddressRecord converts an A record into an address record.
func aToAddressRecord(a *dns.A) addressRecord {
	return addressRecord{
		address:        a.A,
		name:           a.Hdr.Name,
		resourceRecord: headerToResourceRecord(&a.Hdr),
	}
}

// cacheFlushIsSet returns true if the RR's cache flush bit is set.
func cacheFlushIsSet(header *dns.RR_Header) bool {
	const (
		cacheFlushBit = 15 // Highest order bit indicates cache flush (RFC 6762 Section 10.2)
	)

	return (header.Class & (1 << cacheFlushBit)) != 0
}

// headerToResourceRecord converts an RR header into a resource record.
func headerToResourceRecord(header *dns.RR_Header) resourceRecord {
	return resourceRecord{
		cacheFlush: cacheFlushIsSet(header),
		timeToLive: time.Duration(header.Ttl) * time.Second,
	}
}

// newMessagePipeline creates a new, initialized message pipeline
func newMessagePipeline() messagePipeline {
	return messagePipeline{
		addressRecordCh: make(chan addressRecord),
		pointerRecordCh: make(chan pointerRecord),
		serviceRecordCh: make(chan serviceRecord),
		shutdownCh:      make(chan struct{}),
		textRecordCh:    make(chan textRecord),
	}
}

// ptrToPointerRecord converts a PTR record into a pointer record.
func ptrToPointerRecord(ptr *dns.PTR) pointerRecord {
	return pointerRecord{
		instanceName:   ptr.Ptr,
		serviceName:    ptr.Hdr.Name,
		resourceRecord: headerToResourceRecord(&ptr.Hdr),
	}
}

// serviceNameFromInstanceName extracts the service name from the given instance name.
func serviceNameFromInstanceName(instanceName string) string {
	return strings.SplitN(instanceName, ".", 2)[1]
}

// srvToServiceRecord converts an SRV record into a service record.
func srvToServiceRecord(srv *dns.SRV) serviceRecord {
	instanceName := srv.Hdr.Name
	serviceName := serviceNameFromInstanceName(instanceName)

	return serviceRecord{
		instanceName:   instanceName,
		port:           srv.Port,
		serviceName:    serviceName,
		target:         srv.Target,
		resourceRecord: headerToResourceRecord(&srv.Hdr),
	}
}

// txtToMap converts a TXT record to a key-value map.
func txtToMap(txt *dns.TXT) map[string]string {
	values := make(map[string]string)

	for _, value := range txt.Txt {
		kvPair := strings.Split(value, "=")
		if len(kvPair) != 2 {
			// Malformed TXT record
			continue
		}

		values[kvPair[0]] = kvPair[1]
	}

	return values
}

// txtToTextRecord converts a TXT record into a text record.
func txtToTextRecord(txt *dns.TXT) textRecord {
	instanceName := txt.Hdr.Name
	serviceName := serviceNameFromInstanceName(instanceName)

	return textRecord{
		instanceName:   instanceName,
		serviceName:    serviceName,
		values:         txtToMap(txt),
		resourceRecord: headerToResourceRecord(&txt.Hdr),
	}
}

// isIPv4 returns true if the given address record is for an IPv4 address.
func (a *addressRecord) isIPv4() bool {
	return a.address.To4() != nil
}

// close closes the message pipeline.
func (p *messagePipeline) close() {
	go func() {
		p.shutdownCh <- struct{}{}
	}()
}

// onMessageReceived handles receiving the given message.
func (p *messagePipeline) onMessageReceived(msg dns.Msg) {
	if !msg.Response {
		// Don't care about messages that are not questions
		return
	}

	resourceRecords := append(msg.Answer, msg.Extra...)

	for _, rr := range resourceRecords {
		switch resourceRecord := rr.(type) {
		case *dns.A:
			p.addressRecordCh <- aToAddressRecord(resourceRecord)
		case *dns.AAAA:
			p.addressRecordCh <- aaaaToAddressRecord(resourceRecord)
		case *dns.PTR:
			p.pointerRecordCh <- ptrToPointerRecord(resourceRecord)
		case *dns.SRV:
			p.serviceRecordCh <- srvToServiceRecord(resourceRecord)
		case *dns.TXT:
			p.textRecordCh <- txtToTextRecord(resourceRecord)
		}
	}
}

// pipeMessages filters, transforms, and pipes the appropriate messages from the raw DNS message channel into the
// correct output channels.
func (p *messagePipeline) pipeMessages(msgCh <-chan dns.Msg) {
	for {
		select {
		case <-p.shutdownCh:
			return

		case msg := <-msgCh:
			p.onMessageReceived(msg)
		}
	}
}

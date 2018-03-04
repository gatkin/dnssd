package dnssd

import (
	"fmt"
)

// browse browses for service instances on the local network.
func (r *Resolver) browse() {
	defer r.close()

	for {
		select {
		case <-r.shutdownCh:
			return

		case addressRecord := <-r.messagePipeline.addressRecordCh:
			fmt.Printf("%#v\n", addressRecord)
			r.cache.onAddressRecordReceived(addressRecord)

		case pointerRecord := <-r.messagePipeline.pointerRecordCh:
			fmt.Printf("%#v\n", pointerRecord)
			r.cache.onPointerRecordReceived(pointerRecord)

		case serviceRecord := <-r.messagePipeline.serviceRecordCh:
			fmt.Printf("%#v\n", serviceRecord)
			r.cache.onServiceRecordReceived(serviceRecord)

		case textRecord := <-r.messagePipeline.textRecordCh:
			fmt.Printf("%#v\n", textRecord)
			r.cache.onTextRecordReceived(textRecord)
		}
	}
}

// close cleans up all resources owned by the resolver.
func (r *Resolver) close() {
	r.netClient.close()
	r.messagePipeline.close()
}

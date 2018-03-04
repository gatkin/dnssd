package dnssd

import (
	"fmt"
	"log"
)

// browse browses for service instances on the local network.
func (r *Resolver) browse() {
	defer r.close()

	for {
		select {
		case <-r.shutdownCh:
			return

		case addressRecord := <-r.messagePipeline.addressRecordCh:
			if cacheUpdated := r.cache.onAddressRecordReceived(addressRecord); cacheUpdated {
				r.onCacheUpdated()
			}

		case pointerRecord := <-r.messagePipeline.pointerRecordCh:
			if cacheUpdated := r.cache.onPointerRecordReceived(pointerRecord); cacheUpdated {
				r.onCacheUpdated()
			}

		case serviceRecord := <-r.messagePipeline.serviceRecordCh:
			if cacheUpdated := r.cache.onServiceRecordReceived(serviceRecord); cacheUpdated {
				r.onCacheUpdated()
			}

		case textRecord := <-r.messagePipeline.textRecordCh:
			if cacheUpdated := r.cache.onTextRecordReceived(textRecord); cacheUpdated {
				r.onCacheUpdated()
			}

		case serviceName := <-r.serviceAddCh:
			r.onServiceAdded(serviceName)
		}
	}
}

// close cleans up all resources owned by the resolver.
func (r *Resolver) close() {
	r.netClient.close()
	r.messagePipeline.close()

	fmt.Printf("%#v\n\n\n", r.cache)

	for id, r := range r.resolvedInstances {
		fmt.Printf("%v - %v\n", id, r)
	}
}

// onCacheUpdated handles updating the resolver's state whenever the cache has been modified.
func (r *Resolver) onCacheUpdated() {
	r.resolvedInstances = r.cache.toResolvedInstances()
}

// onServiceAdded handles adding a new service to browse for.
func (r *Resolver) onServiceAdded(name string) {
	if r.services[name] {
		// We were already browsing for this service
		return
	}

	r.services[name] = true

	err := r.netClient.sendQuestion(pointerQuestion{serviceName: name})
	if err != nil {
		log.Printf("dnssd: failed sending pointer question: %v", err)
	}
}

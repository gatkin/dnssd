package dnssd

import (
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

		case request := <-r.getResolvedInstancesCh:
			r.onGetResolvedInstances(request)

		case serviceName := <-r.serviceAddCh:
			r.onServiceAdded(serviceName)
		}
	}
}

// close cleans up all resources owned by the resolver.
func (r *Resolver) close() {
	r.netClient.close()
	r.messagePipeline.close()
}

// onCacheUpdated handles updating the resolver's state whenever the cache has been modified.
func (r *Resolver) onCacheUpdated() {
	r.resolvedInstances = r.cache.toResolvedInstances()
}

// onGetResolvedInstances handles a request to get all resolved service instances.
func (r *Resolver) onGetResolvedInstances(request getResolvedInstancesRequest) {
	instances := make([]ServiceInstance, 0, len(r.resolvedInstances))
	for _, instance := range r.resolvedInstances {
		instances = append(instances, instance)
	}

	request.responseCh <- instances
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

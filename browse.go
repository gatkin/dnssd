package dnssd

import (
	"log"
	"time"
)

// browse browses for service instances on the local network.
func (r *Resolver) browse() {
	defer r.close()

	for {
		select {
		case <-r.shutdownCh:
			return

		case addressRecord := <-r.messagePipeline.addressRecordCh:
			r.onAddressRecordReceived(addressRecord)

		case pointerRecord := <-r.messagePipeline.pointerRecordCh:
			r.onPointerRecordReceived(pointerRecord)

		case serviceRecord := <-r.messagePipeline.serviceRecordCh:
			r.onServiceRecordReceived(serviceRecord)

		case textRecord := <-r.messagePipeline.textRecordCh:
			r.onTextRecordReceived(textRecord)

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

// onAddressRecordReceived handles receiving an address record.
func (r *Resolver) onAddressRecordReceived(record addressRecord) {
	r.onTimeElapsed()
	r.cache.onAddressRecordReceived(record)
	r.onCacheUpdated()
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

// onPointerRecordReceived handles receiving a new pointer record.
func (r *Resolver) onPointerRecordReceived(record pointerRecord) {
	r.onTimeElapsed()
	r.cache.onPointerRecordReceived(record)
	r.onCacheUpdated()
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

// onServiceRecordReceived handles receiving a new service record.
func (r *Resolver) onServiceRecordReceived(record serviceRecord) {
	r.onTimeElapsed()
	r.cache.onServiceRecordReceived(record)
	r.onCacheUpdated()
}

// onTextRecordReceived handles receiving a new text record.
func (r *Resolver) onTextRecordReceived(record textRecord) {
	r.onTimeElapsed()
	r.cache.onTextRecordReceived(record)
	r.onCacheUpdated()
}

// onTimeElapsed updates the resolver's cache based on how long it has been since the cache was
// last updated.
func (r *Resolver) onTimeElapsed() {
	now := time.Now()
	duration := now.Sub(r.lastCacheUpdate)

	r.cache.onTimeElapsed(duration)

	r.lastCacheUpdate = now
}

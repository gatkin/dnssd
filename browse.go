package dnssd

import (
	"log"
	"time"
)

// browse browses for service instances on the local network.
func (r *Resolver) browse() {
	defer r.close()

	r.cacheUpdateTimer = timerCreate()

	for {
		select {
		case <-r.shutdownCh:
			return

		case addressRecord := <-r.messagePipeline.addressRecordCh:
			log.Printf("Received address record %v ttl = %v\n", addressRecord.address, addressRecord.timeToLive)
			r.onAddressRecordReceived(addressRecord)

		case pointerRecord := <-r.messagePipeline.pointerRecordCh:
			log.Printf("Received pointer record ttl = %v\n", pointerRecord.timeToLive)
			r.onPointerRecordReceived(pointerRecord)

		case serviceRecord := <-r.messagePipeline.serviceRecordCh:
			log.Printf("Received service record ttl = %v\n", serviceRecord.timeToLive)
			r.onServiceRecordReceived(serviceRecord)

		case textRecord := <-r.messagePipeline.textRecordCh:
			log.Printf("Received text record ttl = %v\n", textRecord.timeToLive)
			r.onTextRecordReceived(textRecord)

		case request := <-r.getResolvedInstancesCh:
			r.onGetResolvedInstances(request)

		case serviceName := <-r.serviceAddCh:
			log.Printf("Adding service %v\n", serviceName)
			r.onServiceAdded(serviceName)

		case <-r.cacheUpdateTimer.C:
			log.Printf("Cache update timer fired\n")
			r.onCacheUpdateTimerFired()
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

	minTimeToLive := r.cache.getMinTimeToLive()
	timerReset(r.cacheUpdateTimer, minTimeToLive)
}

// onCacheUpdateTimerFired handles updating the cache when the cache update timer fires.
func (r *Resolver) onCacheUpdateTimerFired() {
	r.onTimeElapsed()
	r.onCacheUpdated()
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

// timerCreate creates a new timer that will not fire until reset with a new duration.
func timerCreate() *time.Timer {
	timer := time.NewTimer(time.Hour)
	timerStop(timer)
	return timer
}

// timerReset safely resets the given timer.
func timerReset(timer *time.Timer, duration time.Duration) {
	timerStop(timer)
	timer.Reset(duration)
}

// timerStop safely stops the given timer and ensures no values can be read from its channel
func timerStop(timer *time.Timer) {
	if !timer.Stop() {
		// The timer already fired, drain its channel to ensure no values can be read
		// from it.
		select {
		case <-timer.C:
		default:
		}
	}
}

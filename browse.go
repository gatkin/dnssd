package dnssd

import (
	"log"
	"time"
)

const (
	periodicUpdateInterval = time.Second * 1
)

// browse browses for service instances on the local network.
func (r *Resolver) browse() {
	defer r.close()

	r.periodicUpdateTimer = timerCreate()
	timerReset(r.periodicUpdateTimer, periodicUpdateInterval)

	for {
		select {
		case <-r.shutdownCh:
			return

		case answers := <-r.messagePipeline.answerCh:
			r.onAnswersReceived(answers)

		case request := <-r.getResolvedInstancesCh:
			r.onGetResolvedInstances(request)

		case serviceName := <-r.serviceAddCh:
			log.Printf("Adding service %v\n", serviceName)
			r.onServiceAdded(serviceName)

		case <-r.periodicUpdateTimer.C:
			log.Printf("Cache update timer fired\n")
			r.onPeriodicUpdate()
		}
	}
}

// close cleans up all resources owned by the resolver.
func (r *Resolver) close() {
	r.netClient.close()
	r.messagePipeline.close()
}

// onAnswersReceived handles receiving DNS answers.
func (r *Resolver) onAnswersReceived(answers answerSet) {
	// First update all time-to-live values in the cache
	r.onTimeElapsed()

	for _, record := range answers.addressRecords {
		log.Printf("Received address record %v ttl = %v\n", record.address, record.remainingTimeToLive)
		r.cache.onAddressRecordReceived(record)
	}

	for _, record := range answers.pointerRecords {
		log.Printf("Received pointer record %v, ttl = %v\n", record.instanceName, record.remainingTimeToLive)
		r.cache.onPointerRecordReceived(record)
	}

	for _, record := range answers.serviceRecords {
		log.Printf("Received service record %v, ttl = %v\n", record.instanceName, record.remainingTimeToLive)
		r.cache.onServiceRecordReceived(record)
	}

	for _, record := range answers.textRecords {
		log.Printf("Received text record %v, ttl = %v\n", record.instanceName, record.remainingTimeToLive)
		r.cache.onTextRecordReceived(record)
	}

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

// onPeriodicUpdate handles updating the cache when the cache update timer fires.
func (r *Resolver) onPeriodicUpdate() {
	r.onTimeElapsed()
	r.onCacheUpdated()
	r.sendOutstandingQuestions()
	timerReset(r.periodicUpdateTimer, periodicUpdateInterval)
}

// onServiceAdded handles adding a new service to browse for.
func (r *Resolver) onServiceAdded(name serviceName) {
	if r.browseSet[name] {
		// We were already browsing for this service
		return
	}

	r.browseSet[name] = true

	question := question{
		name:         name.String(),
		questionType: questionTypePointer,
	}

	err := r.netClient.sendQuestion(question)
	if err != nil {
		log.Printf("dnssd: failed sending pointer question: %v", err)
	}
}

// onTimeElapsed updates the resolver's cache based on how long it has been since the cache was
// last updated.
func (r *Resolver) onTimeElapsed() {
	now := time.Now()
	duration := now.Sub(r.lastCacheUpdate)

	r.cache.onTimeElapsed(duration)

	r.lastCacheUpdate = now
}

// sendOutstandingQuestions sends all questions needed to resolve the set of services being browsed for based on the
// current state of the cache.
func (r *Resolver) sendOutstandingQuestions() {
	questionSet := make(map[question]bool)
	r.cache.getQuestionsForExpiringRecords(r.browseSet, questionSet)
	r.cache.getQuestionsForMissingRecords(r.browseSet, questionSet)

	questions := make([]question, 0, len(questionSet))
	for q := range questionSet {
		log.Printf("Sending question %v\n", q)
		questions = append(questions, q)
	}

	r.netClient.sendQuestions(questions)
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

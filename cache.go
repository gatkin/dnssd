package dnssd

import (
	"time"
)

// addressRecordID is a unique identifier for an address record.
type addressRecordID struct {
	address string
	name    hostName
}

// cache manages a cache of received resource records.
type cache struct {
	addressRecords map[addressRecordID]addressRecord
	pointerRecords map[serviceInstanceName]pointerRecord
	serviceRecords map[serviceInstanceName]serviceRecord
	textRecords    map[serviceInstanceName]textRecord
}

// serviceInstanceID is a unique identifier for a fully resolved service instance.
type serviceInstanceID struct {
	address string
	name    serviceInstanceName
}

type questionType int

const (
	questionTypeIPv4Address questionType = iota
	questionTypeIPv6Address
	questionTypePointer
	questionTypeService
	questionTypeText
)

type question struct {
	name         string
	questionType questionType
}

// addressRecordsByHostName returns a mapping of address records by host name.
func addressRecordsByHostName(records map[addressRecordID]addressRecord) map[hostName][]addressRecord {
	byName := make(map[hostName][]addressRecord)
	for _, record := range records {
		byName[record.name] = append(byName[record.name], record)
	}

	return byName
}

// pointerRecordsByService returns a mapping of service names to the set of pointer records that
// belong to the service.
func pointerRecordsByService(records map[serviceInstanceName]pointerRecord) map[serviceName][]pointerRecord {
	byService := make(map[serviceName][]pointerRecord)
	for _, record := range records {
		byService[record.serviceName] = append(byService[record.serviceName], record)
	}

	return byService
}

// newCache creates a new DNS cache.
func newCache() cache {
	return cache{
		addressRecords: make(map[addressRecordID]addressRecord),
		pointerRecords: make(map[serviceInstanceName]pointerRecord),
		serviceRecords: make(map[serviceInstanceName]serviceRecord),
		textRecords:    make(map[serviceInstanceName]textRecord),
	}
}

// getID returns the address records unique identifier.
func (a *addressRecord) getID() addressRecordID {
	return addressRecordID{
		address: a.address.String(),
		name:    a.name,
	}
}

// getMinTimeToLive returns the minimum time-to-live for all resource records in the cache.
func (c *cache) getMinTimeToLive() time.Duration {
	minTTL := 1 * time.Hour

	for _, record := range c.addressRecords {
		if record.timeToLive < minTTL {
			minTTL = record.timeToLive
		}
	}

	for _, record := range c.pointerRecords {
		if record.timeToLive < minTTL {
			minTTL = record.timeToLive
		}
	}

	for _, record := range c.serviceRecords {
		if record.timeToLive < minTTL {
			minTTL = record.timeToLive
		}
	}

	for _, record := range c.textRecords {
		if record.timeToLive < minTTL {
			minTTL = record.timeToLive
		}
	}

	return minTTL
}

// getQuestionsForMissingRecords returns the set of questions for records that are missing from the cache
// which are needed to resolve the given set of services that are being browsed for.
func (c *cache) getQuestionsForMissingRecords(browseSet map[serviceName]bool) map[question]bool {
	questions := make(map[question]bool)
	addressRecords := addressRecordsByHostName(c.addressRecords)
	pointerRecords := pointerRecordsByService(c.pointerRecords)

	for serviceName := range browseSet {
		pointers, ok := pointerRecords[serviceName]
		if !ok {
			// Do not continually ask for pointer records. We ask for pointer records when we first start
			// browsing for a particular service. If we do not receive any, then that means there are
			// likely no instances of that service on the network. If an instance of the service does
			// come online at a later time, it should send an announcement of its presence on the network
			// as per RFC 6762 section 8.3
			continue
		}

		for _, pointer := range pointers {
			service, ok := c.serviceRecords[pointer.instanceName]
			if !ok {
				question := question{
					name:         pointer.instanceName.String(),
					questionType: questionTypeService,
				}
				questions[question] = true
			} else {
				if _, ok := addressRecords[service.target]; !ok {
					ipV4Question := question{
						name:         service.target.String(),
						questionType: questionTypeIPv4Address,
					}

					ipV6Question := question{
						name:         service.target.String(),
						questionType: questionTypeIPv6Address,
					}

					questions[ipV4Question] = true
					questions[ipV6Question] = true
				}
			}

			if _, ok := c.textRecords[pointer.instanceName]; !ok {
				question := question{
					name:         pointer.instanceName.String(),
					questionType: questionTypeText,
				}
				questions[question] = true
			}
		}
	}

	return questions
}

// onAddressRecordReceived updates the cache with the given address record. Returns true
// if the cache was actually updated with the new record.
func (c *cache) onAddressRecordReceived(record addressRecord) bool {
	cacheUpdated := false
	id := record.getID()

	existingRecord, ok := c.addressRecords[id]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.addressRecords[id] = record
		cacheUpdated = true
	}

	return cacheUpdated
}

// onPointerRecordReceived updates the cache with the given pointer record. Returns true
// if the cache was actually updated with the new record.
func (c *cache) onPointerRecordReceived(record pointerRecord) bool {
	cacheUpdated := false

	existingRecord, ok := c.pointerRecords[record.instanceName]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.pointerRecords[record.instanceName] = record
		cacheUpdated = true
	}

	return cacheUpdated
}

// onServiceRecordReceived updates the cache with the given service record. Returns true
// if the cache was actually updated with the new record.
func (c *cache) onServiceRecordReceived(record serviceRecord) bool {
	cacheUpdated := false

	existingRecord, ok := c.serviceRecords[record.instanceName]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.serviceRecords[record.instanceName] = record
		cacheUpdated = true
	}

	return cacheUpdated
}

// onTextRecordReceived updates the cache with the given text record. Returns true
// if the cache was actually updated with the new record.
func (c *cache) onTextRecordReceived(record textRecord) bool {
	cacheUpdated := false

	existingRecord, ok := c.textRecords[record.instanceName]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.textRecords[record.instanceName] = record
		cacheUpdated = true
	}

	return cacheUpdated
}

// onTimeElapsed updates the cache based on the specified amount of elapsed time. Any resource
// records whose time-to-live has expired will be evicted from the cache. Returns true if any
// records have been evicted.
func (c *cache) onTimeElapsed(duration time.Duration) bool {
	cacheUpdated := false

	for id, record := range c.addressRecords {
		record.timeToLive -= duration
		if record.timeToLive > 0 {
			c.addressRecords[id] = record
		} else {
			delete(c.addressRecords, id)
			cacheUpdated = true
		}
	}

	for id, record := range c.pointerRecords {
		record.timeToLive -= duration
		if record.timeToLive > 0 {
			c.pointerRecords[id] = record
		} else {
			delete(c.pointerRecords, id)
			cacheUpdated = true
		}
	}

	for id, record := range c.serviceRecords {
		record.timeToLive -= duration
		if record.timeToLive > 0 {
			c.serviceRecords[id] = record
		} else {
			delete(c.serviceRecords, id)
			cacheUpdated = true
		}
	}

	for id, record := range c.textRecords {
		record.timeToLive -= duration
		if record.timeToLive > 0 {
			c.textRecords[id] = record
		} else {
			delete(c.textRecords, id)
			cacheUpdated = true
		}
	}

	return cacheUpdated
}

// toResolvedInstances returns the set of fully resolved service instances in the cache.
func (c *cache) toResolvedInstances() map[serviceInstanceID]ServiceInstance {
	instances := make(map[serviceInstanceID]ServiceInstance)
	addressRecords := addressRecordsByHostName(c.addressRecords)

	for instanceName := range c.pointerRecords {
		serviceRecord, hasService := c.serviceRecords[instanceName]
		if !hasService {
			continue
		}

		textRecord, hasText := c.textRecords[instanceName]
		if !hasText {
			continue
		}

		for _, addressRecord := range addressRecords[serviceRecord.target] {
			instance := ServiceInstance{
				Address:      addressRecord.address,
				InstanceName: instanceName.String(),
				Port:         serviceRecord.port,
				ServiceName:  serviceRecord.serviceName.String(),
				TextRecords:  textRecord.values,
			}

			instances[instance.getID()] = instance
		}
	}

	return instances
}

// getID returns the service instance's unique id.
func (s *ServiceInstance) getID() serviceInstanceID {
	return serviceInstanceID{
		address: s.Address.String(),
		name:    serviceInstanceName(s.InstanceName),
	}
}

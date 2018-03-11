package dnssd

import (
	"time"
)

// addressRecordID is a unique identifier for an address record.
type addressRecordID struct {
	address string
	name    string
}

// cache manages a cache of received resource records.
type cache struct {
	addressRecords map[addressRecordID]addressRecord

	// Maps from instance name to pointer record.
	pointerRecords map[string]pointerRecord

	// Maps from instance name to service record.
	serviceRecords map[string]serviceRecord

	// Maps from instance name to text record.
	textRecords map[string]textRecord
}

// serviceInstanceID is a unique identifier for a fully resolved service instance.
type serviceInstanceID struct {
	address string
	name    string
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

// addressRecordsByName returns a mapping of address records by name.
func addressRecordsByName(records map[addressRecordID]addressRecord) map[string][]addressRecord {
	byName := make(map[string][]addressRecord)
	for _, record := range records {
		byName[record.name] = append(byName[record.name], record)
	}

	return byName
}

// newCache creates a new DNS cache.
func newCache() cache {
	return cache{
		addressRecords: make(map[addressRecordID]addressRecord),
		pointerRecords: make(map[string]pointerRecord),
		serviceRecords: make(map[string]serviceRecord),
		textRecords:    make(map[string]textRecord),
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
	addressRecords := addressRecordsByName(c.addressRecords)

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
				InstanceName: instanceName,
				Port:         serviceRecord.port,
				ServiceName:  serviceRecord.serviceName,
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
		name:    s.InstanceName,
	}
}

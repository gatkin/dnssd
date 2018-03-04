package dnssd

// cache manages a cache of received resource records.
type cache struct {
	// Maps from name to IPv4 address record.
	addressRecordsV4 map[string]addressRecord

	// Maps from name to IPv6 address record.
	addressRecordsV6 map[string]addressRecord

	// Maps from instance name to pointer record.
	pointerRecords map[string]pointerRecord

	// Maps from instance name to service record.
	serviceRecords map[string]serviceRecord

	// Maps from instance name to text record.
	textRecords map[string]textRecord
}

// newCache creates a new DNS cache.
func newCache() cache {
	return cache{
		addressRecordsV4: make(map[string]addressRecord),
		addressRecordsV6: make(map[string]addressRecord),
		pointerRecords:   make(map[string]pointerRecord),
		serviceRecords:   make(map[string]serviceRecord),
		textRecords:      make(map[string]textRecord),
	}
}

// onAddressRecordReceived updates the cache with the given address record.
func (c *cache) onAddressRecordReceived(record addressRecord) {
	var recordSet map[string]addressRecord
	if record.isIPv4() {
		recordSet = c.addressRecordsV4
	} else {
		recordSet = c.addressRecordsV6
	}

	existingRecord, ok := recordSet[record.name]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		recordSet[record.name] = record
	}
}

// onPointerRecordReceived updates the cache with the given pointer record.
func (c *cache) onPointerRecordReceived(record pointerRecord) {
	existingRecord, ok := c.pointerRecords[record.instanceName]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.pointerRecords[record.instanceName] = record
	}
}

// onServiceRecordReceived updates the cache with the given service record.
func (c *cache) onServiceRecordReceived(record serviceRecord) {
	existingRecord, ok := c.serviceRecords[record.instanceName]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.serviceRecords[record.instanceName] = record
	}
}

// onTextRecordReceived updates the cache with the given text record.
func (c *cache) onTextRecordReceived(record textRecord) {
	existingRecord, ok := c.textRecords[record.instanceName]

	if !ok || record.cacheFlush || record.timeToLive > existingRecord.timeToLive {
		c.textRecords[record.instanceName] = record
	}
}

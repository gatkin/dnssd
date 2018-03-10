package dnssd

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockCache struct {
	addressRecords []addressRecord
	pointerRecords []pointerRecord
	serviceRecords []serviceRecord
	textRecords    []textRecord
}

type addAddressRecordTestCase struct {
	record          addressRecord
	initialRecords  []addressRecord
	expectedRecords []addressRecord
}

type timeElapsedTestCase struct {
	duration           time.Duration
	initialCache       mockCache
	expectedAnyEvicted bool
	expectedCache      mockCache
}

func TestAddAddressRecordCacheFlushSet(t *testing.T) {
	newRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: true,
			timeToLive: 60 * time.Second,
		},
	}

	existingRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 120 * time.Second,
		},
	}

	initialRecords := []addressRecord{
		existingRecord,
	}

	expectedRecords := []addressRecord{
		newRecord,
	}

	testCase := addAddressRecordTestCase{
		record:          newRecord,
		initialRecords:  initialRecords,
		expectedRecords: expectedRecords,
	}

	testCase.run(t)
}

func TestAddAddressRecordDifferentAddress(t *testing.T) {
	existingRecord := addressRecord{
		address: net.ParseIP("172.16.6.197"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 90 * time.Second,
		},
	}

	newRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 120 * time.Second,
		},
	}

	initialRecords := []addressRecord{
		existingRecord,
	}

	expectedRecords := []addressRecord{
		existingRecord,
		newRecord,
	}

	testCase := addAddressRecordTestCase{
		record:          newRecord,
		initialRecords:  initialRecords,
		expectedRecords: expectedRecords,
	}

	testCase.run(t)
}

func TestAddAddressRecordEmpty(t *testing.T) {
	record := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 120 * time.Second,
		},
	}

	initialRecords := []addressRecord{}

	expectedRecords := []addressRecord{
		record,
	}

	testCase := addAddressRecordTestCase{
		record:          record,
		initialRecords:  initialRecords,
		expectedRecords: expectedRecords,
	}

	testCase.run(t)
}

func TestAddAddressRecordHigherTTL(t *testing.T) {
	newRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 240 * time.Second,
		},
	}

	existingRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 120 * time.Second,
		},
	}

	initialRecords := []addressRecord{
		existingRecord,
	}

	expectedRecords := []addressRecord{
		newRecord,
	}

	testCase := addAddressRecordTestCase{
		record:          newRecord,
		initialRecords:  initialRecords,
		expectedRecords: expectedRecords,
	}

	testCase.run(t)
}

func TestAddAddressRecordLowerTTL(t *testing.T) {
	newRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 60 * time.Second,
		},
	}

	existingRecord := addressRecord{
		address: net.ParseIP("172.16.6.0"),
		name:    "test_host",
		resourceRecord: resourceRecord{
			cacheFlush: false,
			timeToLive: 120 * time.Second,
		},
	}

	initialRecords := []addressRecord{
		existingRecord,
	}

	expectedRecords := []addressRecord{
		existingRecord,
	}

	testCase := addAddressRecordTestCase{
		record:          newRecord,
		initialRecords:  initialRecords,
		expectedRecords: expectedRecords,
	}

	testCase.run(t)
}

func TestTimeElapsedEvictions(t *testing.T) {
	duration := time.Second * 300

	initialAddresses := []addressRecord{
		addressRecord{
			address: net.ParseIP("172.16.6.0"),
			name:    "test_host",
			resourceRecord: resourceRecord{
				timeToLive: 120 * time.Second,
			},
		},
	}

	initialPointers := []pointerRecord{
		pointerRecord{
			instanceName: "test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 800 * time.Second,
			},
		},
		pointerRecord{
			instanceName: "another test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 300 * time.Second,
			},
		},
	}

	initialCache := mockCache{
		addressRecords: initialAddresses,
		pointerRecords: initialPointers,
	}

	expectedAddresses := []addressRecord{}

	expectedPointers := []pointerRecord{
		pointerRecord{
			instanceName: "test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 500 * time.Second,
			},
		},
	}

	expectedCache := mockCache{
		addressRecords: expectedAddresses,
		pointerRecords: expectedPointers,
	}

	testCase := timeElapsedTestCase{
		initialCache:       initialCache,
		duration:           duration,
		expectedAnyEvicted: true,
		expectedCache:      expectedCache,
	}

	testCase.run(t)
}

func TestTimeElapsedNothingEvicted(t *testing.T) {
	duration := time.Second * 5

	initialAddresses := []addressRecord{
		addressRecord{
			address: net.ParseIP("172.16.6.0"),
			name:    "test_host",
			resourceRecord: resourceRecord{
				timeToLive: 120 * time.Second,
			},
		},
	}

	initialPointers := []pointerRecord{
		pointerRecord{
			instanceName: "test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 800 * time.Second,
			},
		},
		pointerRecord{
			instanceName: "another test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 240 * time.Second,
			},
		},
	}

	initialCache := mockCache{
		addressRecords: initialAddresses,
		pointerRecords: initialPointers,
	}

	expectedAddresses := []addressRecord{
		addressRecord{
			address: net.ParseIP("172.16.6.0"),
			name:    "test_host",
			resourceRecord: resourceRecord{
				timeToLive: 115 * time.Second,
			},
		},
	}

	expectedPointers := []pointerRecord{
		pointerRecord{
			instanceName: "test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 795 * time.Second,
			},
		},
		pointerRecord{
			instanceName: "another test instance._test_service",
			serviceName:  "_test_service",
			resourceRecord: resourceRecord{
				timeToLive: 235 * time.Second,
			},
		},
	}

	expectedCache := mockCache{
		addressRecords: expectedAddresses,
		pointerRecords: expectedPointers,
	}

	testCase := timeElapsedTestCase{
		initialCache:       initialCache,
		duration:           duration,
		expectedAnyEvicted: false,
		expectedCache:      expectedCache,
	}

	testCase.run(t)
}

func (tc *addAddressRecordTestCase) run(t *testing.T) {
	cache := cache{
		addressRecords: addressesToMap(tc.initialRecords),
	}

	cache.onAddressRecordReceived(tc.record)

	expected := addressesToMap(tc.expectedRecords)
	assert.Equal(t, expected, cache.addressRecords)
}

func (tc *timeElapsedTestCase) run(t *testing.T) {
	actualCache := tc.initialCache.toCache()

	actualAnyEvicted := actualCache.onTimeElapsed(tc.duration)

	assert.Equal(t, tc.expectedAnyEvicted, actualAnyEvicted)

	expectedCache := tc.expectedCache.toCache()
	assert.Equal(t, actualCache, expectedCache)
}

func addressesToMap(addresses []addressRecord) map[addressRecordID]addressRecord {
	addrMap := make(map[addressRecordID]addressRecord)
	for _, record := range addresses {
		addrMap[record.getID()] = record
	}

	return addrMap
}

func pointerRecordsToMap(pointers []pointerRecord) map[string]pointerRecord {
	pointerMap := make(map[string]pointerRecord)
	for _, record := range pointers {
		pointerMap[record.instanceName] = record
	}

	return pointerMap
}

func serviceRecordsToMap(records []serviceRecord) map[string]serviceRecord {
	serviceMap := make(map[string]serviceRecord)
	for _, record := range records {
		serviceMap[record.instanceName] = record
	}

	return serviceMap
}

func textRecordsToMap(records []textRecord) map[string]textRecord {
	textMap := make(map[string]textRecord)
	for _, record := range records {
		textMap[record.instanceName] = record
	}

	return textMap
}

func (m *mockCache) toCache() cache {
	return cache{
		addressRecords: addressesToMap(m.addressRecords),
		pointerRecords: pointerRecordsToMap(m.pointerRecords),
		serviceRecords: serviceRecordsToMap(m.serviceRecords),
		textRecords:    textRecordsToMap(m.textRecords),
	}
}

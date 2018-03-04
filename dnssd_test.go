package dnssd

import (
	"testing"
)

func TestAddTwo(t *testing.T) {
	expected := 4
	actual := AddTwo(3, 1)
	if actual != expected {
		t.Errorf("Wanted %v got %v", expected, actual)
	}
}

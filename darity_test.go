package darity

import (
	"testing"
)

func TestAPIVersion(t *testing.T) {
	v, err := APIVersion()
	if err != nil {
		t.Errorf("could not get API version: %q", err.Error())
	}
	t.Logf("APIVersion: %d", v)
}

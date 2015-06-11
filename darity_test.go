// +build linux

// These tests use a mocked KVM virtual device and allow different ioctl
// implementations to be used.

package darity

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestAPIVersion verifies that Client.APIVersion returns the
// KVM API version identified by the Version constant.
func TestAPIVersionKVM(t *testing.T) {
	c := &Client{
		ioctl: func(fd uintptr, request int, argp uintptr) (uintptr, error) {
			if request != kvmGetAPIVersion {
				t.Fatalf("unexpected ioctl request number: %d", request)
			}

			return uintptr(Version), nil
		},
	}

	v, err := c.APIVersion()
	if err != nil {
		t.Errorf("could not get API version: %q", err.Error())
	}

	if want, got := v, Version; want != got {
		t.Fatalf("unexpected KVM API version: %d != %d", want, got)
	}
}

func TestCreateVM(t *testing.T) {
	c := &Client{
		ioctl: func(fd uintptr, request int, argp uintptr) (uintptr, error) {
			if request != kvmCreateVM {
				t.Fatalf("unexpected ioctl request number: %d", request)
			}

			// return something that looks like an fd but wouldn't be
			// std{in,out,err}
			return 3, nil
		},
	}

	_, err := c.CreateVM(MachineTypeDefault)
	if err != nil {
		t.Errorf("could not create vm: %q", err.Error())
	}
}

// tempFile creates a temporary file for use as a mock KVM virtual device, and
// returns the file and a function to clean it up and remove it on completion.
func tempFile(t *testing.T) (*os.File, func()) {
	temp, err := ioutil.TempFile(os.TempDir(), "darity")
	if err != nil {
		t.Fatal(err)
	}

	return temp, func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}
}

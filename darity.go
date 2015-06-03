// Package darity provides a set of functions for interacting with KVM.
//
// +build linux
package darity

// #include <linux/kvm.h>
import "C"

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const (
	// Version is the expected KVM API version, returned by APIVersion.  This
	// version number is taken directly from KVM API documentation, found here:
	// https://www.kernel.org/doc/Documentation/virtual/kvm/api.txt.
	//
	// If KVM does not return this version number when APIVersion is called,
	// no further interactions should be performed.
	Version = 12

	// devKVM is the name of the KVM virtual device.
	devKVM = "/dev/kvm"
)

var (
	// ErrIncorrectVersion is returned when an incorrect KVM version is found
	// when Enabled is called.  From KVM documentation:
	//   Applications should refuse to run if KVM_GET_API_VERSION returns a
	//   value other than 12.
	ErrIncorrectVersion = errors.New("incorrect KVM version")
)

// Client is a KVM client, and can perform actions using the KVM virtual device,
// such as creating, destroying, or querying virtual machines.
type Client struct {
	// TODO(mdlayher): consider creating an interface for this so tests don't
	// need to rely on the actual KVM virtual device.
	kvm *os.File
}

// New returns a new Client, after performing some sanity checks to ensure that
// the KVM virtual device exists and reports the version identified by Version.
//
// If KVM reports a version not equal to Version, ErrIncorrectVersion will be
// returned, and no further actions should be performed.
func New() (*Client, error) {
	// Open KVM virtual device
	kvm, err := os.OpenFile(devKVM, syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	c := &Client{
		kvm: kvm,
	}

	// Verify correct KVM API version
	version, err := c.APIVersion()
	if err != nil {
		_ = c.Close()
		return nil, err
	}

	// Incorrect API version
	if version != Version {
		_ = c.Close()
		return nil, ErrIncorrectVersion
	}

	return c, nil
}

// Close closes the KVM virtual device used by this Client.
func (c *Client) Close() error {
	return c.kvm.Close()
}

// APIVersion returns the current KVM API version, as reported by the KVM
// virtual device.
func (c *Client) APIVersion() (int, error) {
	return ioctl(c.kvm.Fd(), C.KVM_GET_API_VERSION, 0)
}

// ioctl is a wrapper used to perform the ioctl syscall using the input
// file descriptor, request, and arguments pointer.
func ioctl(fd uintptr, request int, argp uintptr) (int, error) {
	ret, _, errnop := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(request),
		argp,
	)
	if errnop != 0 {
		return 0, os.NewSyscallError("ioctl", fmt.Errorf("%d", int(errnop)))
	}
	return int(ret), nil
}

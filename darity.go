// +build linux

// Package darity provides a set of functions for interacting with KVM.
package darity

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

// Constants taken from from <linux/kvm.h>, so cgo is not necessary.
const (
	kvmGetAPIVersion = 44544
	kvmCreateVM      = 44545
)

// MachineType specifies the type of the VM to be created. Paraphrasing the
// KVM API spec at https://www.kernel.org/doc/Documentation/virtual/kvm/api.txt
// "You most certainly want to use [MachineTypeDefault] as the machine type."
type MachineType int

// These constants are derived from <linux/kvm.h> but seem to be absent from the
// tip of master.
const (
	MachineTypeDefault      MachineType = 0
	MachineTypeS390UControl MachineType = 1
	MachineTypePPCHV        MachineType = 1
	MachineTypePPCPR        MachineType = 2
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
	// KVM virtual device
	kvm *os.File

	// ioctl syscall implementation
	ioctl ioctlFunc
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
		// Perform real ioctl syscalls on device
		ioctl: ioctl,
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
	return c.ioctl(c.kvm.Fd(), kvmGetAPIVersion, 0)
}

// CreateVM returns a VM struct built around the fd provided
// by kvm.
func (c *Client) CreateVM(t MachineType) (*VM, error) {
	v, err := c.ioctl(c.kvm.Fd(), kvmCreateVM, uintptr(t))
	if err != nil {
		return nil, err
	}
	return &VM{
		fd:    v,
		ioctl: c.ioctl,
	}, nil
}

// VM is a KVM guest, created by calling CreateVM on the client. It can
// perform actions specified in api.txt as "vm ioctl" such as creating
// VCPUs and setting IRQ lines.
type VM struct {
	// File descriptor of the created VM
	fd int

	// ioctl syscall implementation
	ioctl ioctlFunc
}

// ioctlFunc is the signature for a function which can perform the ioctl syscall,
// or a mocked version of it.
type ioctlFunc func(fd uintptr, request int, argp uintptr) (int, error)

// ioctl is a wrapper used to perform the ioctl syscall using the input
// file descriptor, request, and arguments pointer.
//
// ioctl is the default ioctlFunc implementation, and the one used when New
// is called.
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

// +build linux

// Package darity provides a set of functions for interacting with KVM.
package darity

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
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
	kvmCapNrVCPUS = 9

	kvmGetAPIVersion       = 44544
	kvmCreateVM            = 44545
	kvmCreateVCPU          = 44609
	kvmSetUserMemoryRegion = 1075883590
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

	// ErrTooManyVCPUS is returned when a more than kvmCapMaxCPUS is requested.
	ErrTooManyVCPUS = fmt.Errorf("a maximum of %d VCPUs are supported.", kvmCapNrVCPUS)
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
	v, err := c.ioctl(c.kvm.Fd(), kvmGetAPIVersion, 0)
	return int(v), err
}

// CreateVM returns a VM struct built around the fd provided
// by kvm.
func (c *Client) CreateVM(t MachineType) (*VM, error) {
	v, err := c.ioctl(c.kvm.Fd(), kvmCreateVM, uintptr(t))
	if err != nil {
		return nil, err
	}
	return &VM{
		Memory: make([]*MemorySlot, 0),

		fd:    v,
		ioctl: c.ioctl,
	}, nil
}

// VM is a KVM guest, created by calling CreateVM on the client. It can
// perform actions specified in api.txt as "vm ioctl" such as creating
// VCPUs and setting IRQ lines.
type VM struct {
	// Memory represents a collection of physical memory slots for a VM.
	Memory []*MemorySlot

	// File descriptor of the created VM
	fd uintptr

	// File descriptor for the VM's VCPUs
	vcpufd uintptr

	// ioctl syscall implementation
	ioctl ioctlFunc
}

// MemorySlotFlag is a flag which can be used with VM.AddMemorySlot.
type MemorySlotFlag uint32

// Flags taken from KVM API documentation, Section 4.35.
const (
	MemoryLogDirtyPages MemorySlotFlag = 1
	MemoryReadonly      MemorySlotFlag = 2
)

// MemorySlot represents a virtual memory slot for a guest, and contains metadata
// regarding the memory, as well as the actual backing memory slice.
type MemorySlot struct {
	Slot          uint32
	Flags         uint32
	GuestPhysAddr uint64
	MemorySize    uint64
	UserspaceAddr uint64

	memory []byte
}

// kvmUserspaceMemoryRegion is analagous to kvm_userspace_memory_region, and is
// used to create or modify a guest physical memory slot.
type kvmUserspaceMemoryRegion struct {
	slot          uint32
	flags         uint32
	guestPhysAddr uint64
	memorySize    uint64
	userspaceAddr uint64
}

// AddMemorySlot allocates n bytes of virtual memory for a VM in a single slot,
// using the host's physical memory.  Successive calls can be used to allocate
// multiple slots of virtual memory.
func (v *VM) AddMemorySlot(n uint64, flags MemorySlotFlag) error {
	// Allocate a chunk of memory to be used with a guest
	memory := make([]byte, n)

	// Slot increments with MemorySlots added to the VM
	slot := uint32(len(v.Memory))

	// Physical address starts at 0, and increments by the offset and memory
	// size of the previous slot
	var guestPhysAddr uint64
	if l := len(v.Memory); l > 0 {
		guestPhysAddr = v.Memory[l-1].GuestPhysAddr + v.Memory[l-1].MemorySize
	}

	// TODO: optimize.
	// "It is recommended that the lower 21 bits of guest_phys_addr and userspace_addr
	// be identical.  This allows large pages in the guest to be backed by large
	// pages in the host."

	uFlags := uint32(flags)
	uUserspaceAddr := uint64(uintptr(unsafe.Pointer(&memory[0])))

	// Parameter struct to perform ioctl request
	m := kvmUserspaceMemoryRegion{
		slot:          slot,
		flags:         uFlags,
		guestPhysAddr: guestPhysAddr,
		memorySize:    n,
		userspaceAddr: uUserspaceAddr,
	}

	// Attempt to add a memory slot
	r, err := v.ioctl(v.fd, kvmSetUserMemoryRegion, uintptr(unsafe.Pointer(&m)))
	if err != nil {
		return err
	}
	if r != 0 {
		return errors.New("failed to add memory slot")
	}

	// Store for later use
	v.Memory = append(v.Memory, &MemorySlot{
		Slot:          slot,
		Flags:         uFlags,
		GuestPhysAddr: guestPhysAddr,
		MemorySize:    n,
		UserspaceAddr: uUserspaceAddr,

		// TODO: If we don't keep this here, will the guest's physical memory be
		// garbage collected?
		memory: memory,
	})

	return nil
}

// AddVCPU adds n VCPUs to a virtual machine.
func (v *VM) AddVCPU(n uint64) error {
	if n > kvmCapNrVCPUS {
		return ErrTooManyVCPUS
	}

	r, err := v.ioctl(v.fd, kvmCreateVCPU, uintptr(n))
	if err != nil {
		return err
	}

	v.vcpufd = r

	return nil
}

// ioctlFunc is the signature for a function which can perform the ioctl syscall,
// or a mocked version of it.
type ioctlFunc func(fd uintptr, request int, argp uintptr) (uintptr, error)

// ioctl is a wrapper used to perform the ioctl syscall using the input
// file descriptor, request, and arguments pointer.
//
// ioctl is the default ioctlFunc implementation, and the one used when New
// is called.
func ioctl(fd uintptr, request int, argp uintptr) (uintptr, error) {
	ret, _, errnop := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(request),
		argp,
	)
	if errnop != 0 {
		return 0, os.NewSyscallError("ioctl", fmt.Errorf("%d", int(errnop)))
	}
	return ret, nil
}

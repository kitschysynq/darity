// Package darity provides a set of functions for interacting with kvm
//
// +build linux
package darity

// #include <linux/kvm.h>
import "C"

import (
	"fmt"
	"os"
	"syscall"
)

func ioctl(fd, request int, argp uintptr) (int, error) {
	ret, _, errnop := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(request),
		argp)
	if errnop != 0 {
		return 0, os.NewSyscallError("ioctl", fmt.Errorf("%d", int(errnop)))
	}
	return int(ret), nil
}

func APIVersion() (int, error) {
	fd, err := syscall.Open("/dev/kvm", syscall.O_RDWR|syscall.O_CLOEXEC, 0)
	if err != nil {
		return 0, err
	}

	return ioctl(fd, C.KVM_GET_API_VERSION, 0)
}

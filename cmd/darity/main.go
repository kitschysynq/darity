// Package main provides a toy for playing with darity
package main

import (
	"fmt"

	"github.com/kitschysynq/darity"
)

func main() {
	kvm, err := darity.New()
	if err != nil {
		fmt.Printf("error creating KVM client: %q\n", err.Error())
		return
	}
	defer kvm.Close()

	v, err := kvm.APIVersion()
	if err != nil {
		fmt.Printf("error getting KVM API version: %q\n", err.Error())
		return
	}
	fmt.Printf("KVM API Version: %d\n", v)

	vm, err := kvm.CreateVM(darity.MachineTypeDefault)
	if err != nil {
		fmt.Printf("error creating vm: %q\n", err.Error())
		return
	}

	for i := 0; i < 4; i++ {
		if err := vm.AddMemorySlot(128<<20, 0); err != nil {
			fmt.Printf("error adding memory slot: %q\n", err.Error())
			return
		}

		m := vm.Memory[i]
		fmt.Printf("memory: slot: %02d, size: %d, offset: %d\n", m.Slot, m.MemorySize, m.GuestPhysAddr)
	}
}

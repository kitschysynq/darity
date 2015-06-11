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

	_, err = kvm.CreateVM(darity.KVMMachineTypeDefault)
	if err != nil {
		fmt.Printf("error creating vm: %q\n", err.Error())
	}
}

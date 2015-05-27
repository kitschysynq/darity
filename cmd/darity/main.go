// Package main provides a toy for playing with darity
package main

import (
	"fmt"

	"github.com/kitschysynq/darity"
)

func main() {
	v, err := darity.APIVersion()
	if err != nil {
		fmt.Printf("error getting KVM API version: %q\n", err.Error())
		return
	}
	fmt.Printf("KVM API Version: %d\n", v)
}

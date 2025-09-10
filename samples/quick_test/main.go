package main

import (
	"fmt"
	"reflect"
)

type ITest interface {
	Do() string
}

type Test struct{}

func (t *Test) Do() string {
	return "test"
}

func main() {
	var i ITest = &Test{} // This should work
	fmt.Println(i.Do())

	// Test reflection
	interfaceType := reflect.TypeOf((*ITest)(nil)).Elem()
	concreteType := reflect.TypeOf(&Test{})

	fmt.Printf("Interface: %s\n", interfaceType.String())
	fmt.Printf("Concrete: %s\n", concreteType.String())
	fmt.Printf("Implements: %v\n", concreteType.Implements(interfaceType))
}

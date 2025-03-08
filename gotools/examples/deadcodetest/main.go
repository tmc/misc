package main

import "fmt"

type UnusedInterface interface {
	Unused()
}

type UsedInterface interface {
	Used()
}

type Implementation struct{}

func (i Implementation) Used() {
	fmt.Println("Used method called")
}

func (i Implementation) Unused() {
	fmt.Println("Unused method exists but is never called")
}

func main() {
	var i UsedInterface = Implementation{}
	i.Used() // Only calls the Used method
}
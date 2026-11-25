package main

import (
	"log"
	"os"
)

func helper() {
	log.Fatal("x") // want `log.Fatal outside`
}

func main() {
	panic("x") // want `built-in panic`
	log.Fatal("ok")
	os.Exit(1)
	func() { os.Exit(2) }()
}

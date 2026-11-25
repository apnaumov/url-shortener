package a

import (
	"log"
	"os"
)

func f() {
	panic("x")      // want `built-in panic`
	log.Fatal("x")  // want `log.Fatal outside`
	log.Fatalf("x") // want `log.Fatalf outside`
	os.Exit(1)      // want `os.Exit outside`
	log.Println("ok")
}

func main() {
	os.Exit(1) // want `os.Exit outside`
}

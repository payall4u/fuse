package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"syscall"
)

var (
	input  string
	output string
)

func init() {
	flag.StringVar(&input, "if", "", "")
	flag.StringVar(&output, "of", "", "")
	flag.Parse()
}

func main() {
	if input == "" || output == "" {
		fmt.Println("godd --if=<filename> --of=<filename>")
		os.Exit(1)
	}

	iff, err := os.OpenFile(input, os.O_RDONLY, 0666)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	off, err := os.OpenFile(output, os.O_WRONLY | os.O_APPEND, 0666)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	buf := make([]byte, 4096, 4096)
	n, err := io.CopyBuffer(off, iff, buf)
	if err != nil {
		fmt.Println(err.Error())
	}
	syscall.Sync()
	fmt.Println("write len: ", n)
}
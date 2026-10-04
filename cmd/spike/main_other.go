//go:build !windows && !darwin

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "The native Spike supports Windows and macOS. Use npm run dev for the browser interaction lab on this platform.")
	os.Exit(1)
}

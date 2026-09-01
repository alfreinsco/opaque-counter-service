package main

import (
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	n := flag.Int("n", 1, "number of tokens")
	bytesLen := flag.Int("bytes", 24, "random bytes per token")
	flag.Parse()

	if *n < 1 || *n > 1000 || *bytesLen < 16 || *bytesLen > 64 {
		fmt.Fprintln(os.Stderr, "invalid arguments")
		os.Exit(2)
	}

	for i := 0; i < *n; i++ {
		buf := make([]byte, *bytesLen)
		if _, err := rand.Read(buf); err != nil {
			fmt.Fprintln(os.Stderr, "failed to generate token")
			os.Exit(1)
		}
		fmt.Println(base64.RawURLEncoding.EncodeToString(buf))
	}
}

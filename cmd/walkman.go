// walks directory and prints every file to stdout
package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/abiiranathan/walkman"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <dirname>\n", os.Args[0])
	}

	dir, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalf("can not create absolute path: %v\n", err)
	}

	wm := walkman.New()
	hashes, err := wm.Walk(dir)
	if err != nil {
		log.Fatal(err)
	}

	buf := bytes.Buffer{}
	for _, f := range hashes.ToSlice() {
		buf.WriteString(f.Path)
		buf.WriteString("\n")
	}

	io.Copy(bufio.NewWriter(os.Stdout), &buf)
}

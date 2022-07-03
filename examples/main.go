package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

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

	fmt.Printf("Processing root dir: %s\n", dir)
	wm := walkman.New()
	hashes, err := wm.Walk(dir)
	if err != nil {
		log.Fatal(err)
	}

	pdfFiles := func(file walkman.File) bool {
		if strings.HasSuffix(file.Path, ".pdf") {
			return true
		}
		return false
	}

	sizeGreaterThan := func(size int64) walkman.PathFilter {
		return func(file walkman.File) bool {
			return file.Stats.Size() > size
		}
	}

	// Pdf files greater than 10MB
	bigPdfs := hashes.Filter(pdfFiles, sizeGreaterThan(20*1000*1000))

	for _, fileList := range bigPdfs {
		for _, f := range fileList {
			fmt.Println(f.Path, f.Stats.Size())
		}
	}

	// Print duplicates
	// hashes was not modified by filter above
	buf := bytes.Buffer{}
	for hash, fileList := range hashes {
		if len(fileList) > 1 {
			buf.WriteString(fmt.Sprintf("%s--->%d files\n", hash, len(fileList)))

			for _, f := range fileList {
				buf.WriteString("    " + f.Path + "\n")
			}

			buf.WriteString("\n")
		}
	}

	f, err := os.Create("log.txt")
	if err != nil {
		log.Fatal(err)
	}

	io.Copy(f, &buf)
}

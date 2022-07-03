# walkman

Concurrent implementation of file system traversal(walk).

Uses a fixed number of workers together with a counting
semaphore syncronization pattern to co-ordinate concurrent file system traversal
on top of filepath.WalkDir.

**walkman** uses no external dependencies.

## Installation

- As Library
  
```bash
go get github.com/abiiranathan/walkman
```

- As binary
```bash
go install github.com/abiiranathan/walkman/cmd/walkman@latest
```

### Usage
See [examples](examples/main.go) for usage.

#### API
wakman exposes a simple API.

Initialize a new walkman instance.
```go
wm := walkman.New()
// You can pass options for setting 
// number of workers, verbosity, 
// directories to skip to the constructor.

pathMap := wm.Walk("/home/nabiizy")
fileList := pathMap.ToSlice()

// implemetation for a walkman.PathFilter
pdfFiles := func(file walkman.File) bool{
  if strings.HasSuffix(file.Path, ".pdf") {
    return true
  }
  return false
}

// Filter files with a walkman.PathFilter
pdfMap := pathMap.Filter(pdfFiles)

// Flatten to Slice
pdfList := pdfMap.ToSlice()

```

#### Contributing
Feel free to submit pull requests or issues.

#### Confession
I do not know how to write tests for a File System Interface and I would love your help.

##### Credits
This project was inspired by a YouTube video by Matt Holday and [Github Project](https://github.com/matt4biz/go-class-walk) on concurrency.

#### TODO
1.  Add context.Context and context cancellation if walkman.Walk function takes too long.
2.  Add gopher(why not) to project
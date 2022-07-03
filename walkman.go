// Concurrent implementation of file system traversal(walk).
//
// Uses a fixed number of workers together with a counting
// semaphore to co-ordinate concurrent file system traversal
// on top of filepath.WalkDir.
//
// walkman uses no external dependencies.
package walkman

import (
	"crypto/md5"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Very big directories you may not control
// and dot files.
//
// Especially if you are traversing the home directory.
var dirs_to_skip = []string{
	"AndroidStudioProjects",
	"Android",
	"NetBeansProjects",
	"node_modules",
	"Qt",
	"VirtualBoxVMs",
	"vmime",
	"venv",
	"env",
	"RUST",
	"nltk_data",
	"qt5",
	"qt6",
	"wasm32-unknown-unknown",
}

// harsher is a function that takes in a path to a file
// uses some algorithm to generate a unique hash that can be used
// to identify duplicate files.
//
// You can use the a concatenation of file's basename & size for speed
// to match files with same name and size.
//
// For content hash, walkman.Md5ContentHasher
type harsher func(path string) pair

type config struct {
	verbose       bool
	skip          []string
	noDefaultSkip bool // Instructs walkman to not ignore any directories like .git, .venv,.env,AndroidStudioProjects, etc
}

type option func(*Walkman)

// Syncronises the filepath.WalkDir so that each subdir
// is traversed in parraller by workers.
//
// limits acts as a counting semaphore.
// All pairs are passed onto the results channel when all
// workers are done.
type Walkman struct {
	workers int             // number of workers, default 4*runtime.GOMAXPROCS(0)
	limits  chan bool       // counting semaphore channel
	pairs   chan pair       // channel of pairs(hash to filepath)
	result  chan results    // Channel of Results map
	wg      *sync.WaitGroup // pointer because when wg is copied, it won't work.

	config   *config // control verbosity and filtering operations
	hashFunc harsher // defaults to walkman.NameHarsher
}

type pair struct {
	hash string
	path string
}

type File struct {
	Path  string
	Stats os.FileInfo
}

type fileList []File
type results map[string]fileList

func New(options ...option) *Walkman {
	workers := 2 * runtime.GOMAXPROCS(0)

	wm := &Walkman{
		workers:  workers,
		limits:   make(chan bool, workers),
		pairs:    make(chan pair),
		result:   make(chan results),
		wg:       new(sync.WaitGroup),
		hashFunc: nameHasher,
		config: &config{
			verbose:       false,
			skip:          dirs_to_skip,
			noDefaultSkip: false,
		},
	}

	for _, op := range options {
		op(wm)
	}

	return wm
}

// Pass this option to constructor to turn on verbose mode
func Verbose() option {
	return func(wm *Walkman) {
		wm.config.verbose = true
	}
}

// Pass this function to constructor with extra folder names to skip
func SkipDirs(dirs []string) option {
	return func(wm *Walkman) {
		wm.config.skip = append(wm.config.skip, dirs...)
	}
}

// Modify number of workers
func WithWorkers(n int) option {
	return func(w *Walkman) {
		w.workers = n
	}
}

// modify the harsher function to uniquely idendify each file.
func WithHasher(hashFunc harsher) option {
	return func(w *Walkman) {
		w.hashFunc = hashFunc
	}
}

// Returns true if string v is in s slice
func slice_contains(s []string, v string) bool {
	for _, item := range s {
		if item == v {
			return true
		}
	}
	return false
}

// All folders are included with this option
// except those otherwise specified for exclusion by the caller
// by passing SkipDirs option to the constructor.
func NoDefaultSkip() option {
	return func(w *Walkman) {
		w.config.noDefaultSkip = true

		// remove default skipped dirs
		skipList := []string{}

		for _, f := range w.config.skip {
			if !slice_contains(dirs_to_skip, f) {
				skipList = append(skipList, f)
			}
		}

		w.config.skip = skipList
	}
}

// filename+size implementation of walkman.Hasher
//
// hash := fmt.Sprintf("%s-%d", basename, size)
//
// This the default harsher function
func nameHasher(path string) pair {
	b := filepath.Base(path)

	// Skip error for now
	stat, err := os.Stat(path)
	if err != nil {
		log.Fatal(err)
	}

	fn := fmt.Sprintf("%s-%d", b, stat.Size())

	return pair{hash: fn, path: path}
}

// md5 implementation of walkman.Hasher
func md5ContentHasher(path string) pair {
	file, err := os.Open(path)

	if err != nil {
		log.Fatal(err)
	}

	defer file.Close()

	hash := md5.New() // fast & good enough for small directories

	if _, err := io.Copy(hash, file); err != nil {
		log.Fatal(err)
	}

	return pair{hash: fmt.Sprintf("%x", hash.Sum(nil)), path: path}
}

// Recursively walks dir, calling processFile for regular files that are not empty.
//
// All subdirectories are walked in seperate go routines by
// recursively calling searchTree on the subdirctories.
// Returns a map of files or an error
//
// TODO: Add context cancellation
func (wm *Walkman) Walk(dir string) (results, error) {
	// we need another goroutine so we don't block here
	go wm.collectHashes()

	// multi-threaded walk of the directory tree; we need a
	// waitGroup because we don't know how many to wait for
	wm.wg.Add(1)

	err := wm.searchTree(dir)

	if err != nil {
		return results{}, err
	}

	// we must close the paths channel so the workers stop
	wm.wg.Wait()

	// by closing pairs we signal that all the hashes
	// have been collected; we have to do it here AFTER
	// all the workers are done
	close(wm.pairs)

	return <-wm.result, nil
}

// worker processes each file in this routine by hasing file at path
// sends on on the send-only channel pairs.
func (wm *Walkman) processFile(path string) {
	defer wm.wg.Done()

	// Wait on semaphore
	wm.limits <- true

	// Decrement counter when function completes
	defer func() {
		<-wm.limits
	}()

	wm.pairs <- wm.hashFunc(path)
}

// Loops over the pairs channel, appending all hashes to the results channel when done.
// pairs chan: read only, results chan write-only.
func (wm *Walkman) collectHashes() {
	hashes := make(results)

	for p := range wm.pairs {
		stats, err := os.Stat(p.path)
		if err == nil {
			// No need for locks/mutexes when writing.
			// Channels guarantee proper syncronisation.
			hashes[p.hash] = append(hashes[p.hash], File{Path: p.path, Stats: stats})
		}
	}

	wm.result <- hashes
}

func log_skipped(name string) {
	fmt.Printf("Skipping directory %q\n", name)
}

// Recursively walks dir, calling processFile for regular files
// that are not empty.
//
// All subdirectories are walked in seperate go routines
// by recursively calling searchTree on the subdirctories.
// pairs: write-only chan
func (wm *Walkman) searchTree(dirname string) error {
	defer wm.wg.Done()

	// Skips a folder if name in folders to skip
	skipFolder := func(name string) bool {
		var skip bool

		for _, i := range wm.config.skip {
			if name == i {
				skip = true
				break
			}
		}
		return skip
	}

	// filepath.WalkDirFunc more performant than filepath.WalkFunc
	visitor := func(path string, d fs.DirEntry, err error) error {
		if err != nil && err != os.ErrNotExist {
			return err
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}

		name := fi.Name()

		// Ignore hidden folders and wm.config.skip dirs
		if fi.Mode().IsDir() && (strings.HasPrefix(name, ".") || skipFolder(name)) {
			if wm.config.verbose {
				log_skipped(name)
			}

			return filepath.SkipDir
		}

		// ignore dir itself to avoid an infinite loop!
		if fi.Mode().IsDir() && path != dirname {
			wm.wg.Add(1)

			go wm.searchTree(path)

			if wm.config.verbose {
				fmt.Printf("Processing subdirectory: %q\n", name)
			}

			return filepath.SkipDir
		}

		if fi.Mode().IsRegular() && fi.Size() > 0 {
			wm.wg.Add(1)
			go wm.processFile(path)

			if wm.config.verbose {
				fmt.Printf("Processing file: %q\n", path)
			}
		}

		return nil
	}

	// Wait on semaphore
	wm.limits <- true

	// Decrement semaphore counter when function exits
	defer func() {
		<-wm.limits
	}()

	return filepath.WalkDir(dirname, visitor)
}

// Path filter is called for each path in the map values
// If it returns true, the path is included.
type PathFilter func(file File) bool

// Filter results based on file. Returns a copy of results.
// Warning: This is potentially very expensive if filterFuncs are many
// and doing a lot of work esp IO work.
func (hashes results) Filter(filterFuncs ...PathFilter) results {
	results := make(results)

	for hash, files := range hashes {
		// Loop through all duplicates
		for _, file := range files {
			include := true

			// Apply filter for each path
			for _, filterFunc := range filterFuncs {

				// If we all any filter, we ignore this path
				if !filterFunc(file) {
					include = false
					break
				}
			}

			if include {
				results[hash] = append(results[hash], file)
			}
		}
	}

	return results
}

// Loops over the hashes map and flattens it into a slice of File objects.
func (hashes results) ToSlice() fileList {
	files := []File{}

	for _, fl := range hashes {
		for _, f := range fl {
			files = append(files, File{Path: f.Path, Stats: f.Stats})
		}
	}

	return files
}

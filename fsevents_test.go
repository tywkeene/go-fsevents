package fsevents_test

import (
	"fmt"
	"path"
	//"fmt"
	"runtime"
	"testing"

	fsevents "github.com/tywkeene/go-fsevents"
)

func eq(t *testing.T, compare bool) {
	if compare == false {
		_, filepath, line, _ := runtime.Caller(1)
		_, file := path.Split(filepath)
		t.Fatalf("Comparison @ (file: %s -> line: %d) false", file, line)
	}
}

var testMask int = fsevents.DirCreatedEvent
var testRootDir string = "./"

func TestNewWatcher(t *testing.T) {
	testOptions := &fsevents.WatcherOptions{
		Recursive:       false,
		UseWatcherFlags: false,
	}
	w, err := fsevents.NewWatcher(testRootDir, testMask, testOptions)
	eq(t, (w != nil))
	eq(t, (err == nil))

	for _, d := range w.ListDescriptors() {
		fmt.Println(d)
	}
}

func TestAddDescriptor(t *testing.T) {
	testOptions := &fsevents.WatcherOptions{
		Recursive:       false,
		UseWatcherFlags: false,
	}
	var w *fsevents.Watcher
	var err error

	w, err = fsevents.NewWatcher(testRootDir, testMask, testOptions)
	eq(t, (w != nil))
	eq(t, (err == nil))

	// err should be != nil if we add watch that already exists
	err = w.AddDescriptor(testRootDir, -1)
	eq(t, (err != fsevents.ErrDescAlreadyExists))

	// err should be != nil if directory does not exist
	err = w.AddDescriptor("not_there/", -1)
	eq(t, (err.Error() == fmt.Errorf("%s: %s", fsevents.ErrDescNotCreated, "directory does not exist").Error()))
}

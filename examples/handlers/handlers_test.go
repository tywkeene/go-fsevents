package main

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/spf13/afero"
	fsevents "github.com/tywkeene/go-fsevents"
)

func eq(t *testing.T, compare bool, err error) {
	if compare == false {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			t.Logf("Comparison @ line: %d false\n", line)
			t.Fatal("Error returned:", err)
		} else {
			t.Logf("Comparison @ line: %d false\n", line)
			t.Fatal("Exiting")
		}
	}
}

func TestRegisterEventHandler(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	fs := afero.NewOsFs()
	fs.Mkdir("test", 0644)

	w, err = fsevents.NewWatcher("test", fsevents.AllEvents)
	eq(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	eq(t, (err == nil), err)

	fs.RemoveAll("test")
}

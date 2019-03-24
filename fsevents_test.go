package fsevents_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"testing"
	"time"

	fsevents "github.com/tywkeene/go-fsevents"
)

type MaskTest struct {
	UnixMask uint32
	Args     []string
	Setup    func(...string) error
	Action   string
}

var MaskTests = []MaskTest{
	{fsevents.MovedFrom, []string{"./test/move-test-file", "./test2/moved-file"}, setupRandomFile, "Move from"},
	{fsevents.MovedTo, []string{"./test2/move-test-file", "./test/moved-file"}, setupRandomFile, "Move to"},
	{fsevents.Delete, []string{"./test/delete-file"}, setupRandomFile, "Delete"},
	{fsevents.Open, []string{"./test/open-file-test"}, setupRandomFile, "Open"},
	{fsevents.Modified, []string{"./test/modify-file-test"}, setupRandomFile, "Modified"},
	{fsevents.Accessed, []string{"./test/accessed-file-test"}, setupRandomFile, "Accessed"},
	{fsevents.AttrChange, []string{"./test/attr-change-file-test"}, setupRandomFile, "Attribute changed"},
	{fsevents.CloseWrite, []string{"./test/close-write-file-test"}, setupRandomFile, "Close write"},
	{fsevents.CloseRead, []string{"./test/close-read-file-test"}, setupRandomFile, "Close read"},
	{fsevents.Move, []string{"./test/move-file-test", "./test/move-file-test2"}, setupRandomFile, "Move"},
	{fsevents.Create, []string{"./test/create-file-test"}, setupNothing, "Create"},
	/*
		{fsevents.RootDelete, []string{"./test"}, setupNothing, "Root delete"},
		{fsevents.RootMove, []string{"./test", "/tmp/test-moved"}, setupNothing, "Root move"},
	*/
}

var (
	testRootDir  string = "./test"
	testRootDir2 string = "./test2"
)

var Actions = map[string]func(...string) error{
	"Move to": func(args ...string) error {
		return os.Rename(args[0], args[1])
	},
	"Move from": func(args ...string) error {
		return os.Rename(args[0], args[1])
	},
	"Delete": func(args ...string) error {
		return os.Remove(args[0])
	},
	"Copy": func(args ...string) error {
		return nil
	},
	"Modified": func(args ...string) error {
		fd, err := os.OpenFile(args[0], os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer fd.Close()

		fd.Write([]byte("test"))

		return nil
	},
	"Accessed": func(args ...string) error {
		fd, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer fd.Close()

		var buffer []byte = make([]byte, 1)
		_, err = fd.ReadAt(buffer, 1)

		return err
	},
	"Attribute changed": func(args ...string) error {
		return os.Chmod(args[0], 0644)
	},
	"Open": func(args ...string) error {
		_, err := os.Open(args[0])
		return err
	},
	"Close write": func(args ...string) error {
		return writeRandomFile(args[0])
	},
	"Close read": func(args ...string) error {
		fd, err := os.Open(args[0])
		if err != nil {
			return err
		}
		fd.Close()
		return nil
	},
	"Move": func(args ...string) error {
		return os.Rename(args[0], args[1])
	},
	"Create": func(args ...string) error {
		return writeRandomFile(args[0])
	},
	"Root delete": func(args ...string) error {
		return os.RemoveAll(args[0])
	},
	"Root move": func(args ...string) error {
		return os.Rename(args[0], args[1])
	},
}

func setupTestDirectories(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) == false {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0777); err != nil {
		return err
	}
	return nil
}

func removeTestDirectories() {
	os.RemoveAll(testRootDir)
	os.RemoveAll(testRootDir2)
}

func eq(t *testing.T, compare bool, err error) {
	if compare == false {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			t.Logf("Comparison @ line: %d false\n", line)
			removeTestDirectories()
			t.Fatal("Error returned:", err)
		} else {
			t.Logf("Comparison @ line: %d false\n", line)
			removeTestDirectories()
			t.Fatal("Exiting")
		}
	}
}

func writeRandomFile(path string) error {
	var random = []byte("ABCDEF1234567890")
	rand.Seed(time.Now().UnixNano())
	buffer := make([]byte, 16)
	for i := range buffer {
		buffer[i] = random[rand.Intn(len(random))]
	}

	if err := ioutil.WriteFile(path, buffer, 0644); err != nil {
		return err
	}
	return nil
}

func setupRandomFile(args ...string) error {
	return writeRandomFile(args[0])
}

func setupNothing(args ...string) error {
	return nil
}

func TestMasks(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	setupTestDirectories(testRootDir)
	setupTestDirectories(testRootDir2)
	defer removeTestDirectories()

	for _, maskTest := range MaskTests {

		err = maskTest.Setup(maskTest.Args...)
		eq(t, (err == nil), err)

		w, err = fsevents.NewWatcher(testRootDir, maskTest.UnixMask)
		eq(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
		eq(t, (err == nil), err)

		w.StartAll()

		testFunc := Actions[maskTest.Action]
		err = testFunc(maskTest.Args...)
		eq(t, (err == nil), err)

		event, err := w.ReadSingleEvent()
		eq(t, (err == nil), err)

		// Ensure the event and its data is consistent
		eq(t, (event != nil), fmt.Errorf("ReadSingleEvent should have returned a non-nil event"))
		eq(t, (event.Name != ""), fmt.Errorf("The Name field in the event should not be empty"))
		eq(t, (event.Path != ""), fmt.Errorf("The Name field in the event should not be empty"))

		eq(t, (w.GetEventCount() == 1), nil)
		eq(t, (fsevents.CheckMask(maskTest.UnixMask, event.RawEvent.Mask) == true),
			fmt.Errorf("Event returned invalid mask: Expected: %d Got: %d\n", maskTest.UnixMask, event.RawEvent.Mask))

		w.StopAll()
		w.RemoveDescriptor(testRootDir)
	}
}

func TestNewWatcher(t *testing.T) {

	setupTestDirectories(testRootDir)
	defer removeTestDirectories()

	w, err := fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	eq(t, (err == nil), err)
	eq(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
	eq(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))
}

func TestStart(t *testing.T) {
	setupTestDirectories(testRootDir)
	defer removeTestDirectories()

	w, err := fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	eq(t, (err == nil), err)
	eq(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
	eq(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	d := w.GetDescriptorByPath(w.RootPath)
	eq(t, (d != nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))

	d.Start()
	eq(t, (w.GetRunningDescriptors() == 1), fmt.Errorf("GetRunningDescriptor should have returned 1"))
}

func TestAddDescriptor(t *testing.T) {

	setupTestDirectories(testRootDir)
	defer removeTestDirectories()

	var w *fsevents.Watcher
	var err error

	w, err = fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	eq(t, (err == nil), err)
	eq(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	eq(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	// AddDescriptor SHOULD return ErrDescNotCreated if we try to add a WatchDescriptor for a directory that does not exist
	d, err := w.AddDescriptor("not_there/", fsevents.AllEvents)
	eq(t, (d == nil), fmt.Errorf("AddDescriptor should have returned nil descriptor"))
	expectedErr := fmt.Errorf("%s: %s", fsevents.ErrDescNotCreated, "directory does not exist").Error()
	eq(t, (err.Error() == expectedErr), err)

	// AddDescriptor SHOULD return a non-nil WatchDescriptor and a non-nil error if we add a WatchDescriptor for a directory
	// that exists on disk and does not already have a running watch
	d, err = w.AddDescriptor(testRootDir, fsevents.AllEvents)
	eq(t, (d != nil), fmt.Errorf("AddDescriptor should have returned non-nil descriptor"))
	eq(t, (err != fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))

	// AddDescriptor SHOULD NOT return error if we add a WatchDescriptor for a directory that exists and is not already watched
	d, err = w.AddDescriptor(testRootDir, fsevents.AllEvents)
	eq(t, (d == nil), fmt.Errorf("AddDescriptor should have returned nil descriptor"))
	eq(t, (err == fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))
}

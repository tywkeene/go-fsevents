package fsevents_test

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"testing"
	"time"

	fsevents "github.com/tywkeene/go-fsevents"
	"golang.org/x/sys/unix"
)

type MaskTest struct {
	Mask   uint32
	Args   []string
	Setup  func(...string) error
	Action func(...string) error
}

var MaskTests = map[string]MaskTest{
	"MovedFrom": {
		Mask:   fsevents.MovedFrom,
		Args:   []string{"./test/move-test-file", "./test2/moved-file"},
		Setup:  writeToFile,
		Action: move,
	},
	"MovedTo": {
		Mask:   fsevents.MovedTo,
		Args:   []string{"./test2/move-test-file", "./test/moved-file"},
		Setup:  writeToFile,
		Action: move,
	},
	"Delete": {
		Mask:   fsevents.Delete,
		Args:   []string{"./test/delete-file"},
		Setup:  writeToFile,
		Action: remove,
	},
	"Open": {
		Mask:   fsevents.Open,
		Args:   []string{"./test/open-file-test"},
		Setup:  writeToFile,
		Action: open,
	},
	"Modified": {
		Mask:   fsevents.Modified,
		Args:   []string{"./test/modify-file-test"},
		Setup:  writeToFile,
		Action: modify,
	},
	"Accessed": {
		Mask:   fsevents.Accessed,
		Args:   []string{"./test/accessed-file-test"},
		Setup:  writeToFile,
		Action: access,
	},
	"AttrChange": {
		Mask:   fsevents.AttrChange,
		Args:   []string{"./test/attr-change-file-test"},
		Setup:  writeToFile,
		Action: changeAttr,
	},
	"CloseWrite": {
		Mask:   fsevents.CloseWrite,
		Args:   []string{"./test/close-write-file-test"},
		Setup:  writeToFile,
		Action: writeToFile,
	},
	"CloseRead": {
		Mask:   fsevents.CloseRead,
		Args:   []string{"./test/close-read-file-test"},
		Setup:  writeToFile,
		Action: access,
	},
	"Move": {
		Mask:   fsevents.Move,
		Args:   []string{"./test/move-file-test", "./test/move-file-test2"},
		Setup:  writeToFile,
		Action: move,
	},
	"Create": {
		Mask:   fsevents.Create,
		Args:   []string{"./test/create-file-test"},
		Setup:  nothing,
		Action: writeToFile,
	},
	/*
		{fsevents.RootDelete, []string{"./test"}, nothing, "Root delete"},
		{fsevents.RootMove, []string{"./test", "/tmp/test-moved"}, nothing, "Root move"},
	*/
}

var (
	testRootDir  string = "./test"
	testRootDir2 string = "./test2"
)

func open(args ...string) error {
	_, err := os.Open(args[0])
	return err
}

func modify(args ...string) error {
	fd, err := os.OpenFile(args[0], os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer fd.Close()

	fd.Write([]byte("test"))

	return nil
}

func access(args ...string) error {
	fd, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer fd.Close()

	var buffer []byte = make([]byte, 1)
	_, err = fd.ReadAt(buffer, 1)

	return err
}

func changeAttr(args ...string) error { return os.Chmod(args[0], 0644) }

func move(args ...string) error { return os.Rename(args[0], args[1]) }

func remove(args ...string) error { return os.RemoveAll(args[0]) }

func writeToFile(args ...string) error { return writeRandomFile(args[0]) }

func nothing(args ...string) error { return nil }

func assert(t *testing.T, compare bool, err error) {
	if compare == false {
		_, _, line, _ := runtime.Caller(1)
		t.Logf("Comparison @ [line %d] failed\n", line)
		if err != nil {
			t.Fatal("Error returned:", err)
		} else {
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

	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0777)
	if err != nil {
		return err
	}
	defer fd.Close()

	n, err := fd.Write(buffer)
	if n < len(buffer) {
		return fmt.Errorf("Wrote %d of %d bytes to file %q", n, len(buffer), path)
	}

	return err
}

func setupDirs(paths []string) error {
	for _, path := range paths {
		if err := os.Mkdir(path, 0777); err != nil {
			return fmt.Errorf("Failed to create directory %q: %s\n", path, err.Error())
		}
	}
	return nil
}

func teardownDirs(paths []string) error {
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("Failed to remove directory %q: %s\n", path, err.Error())
		}
	}
	return nil
}

func TestMasks(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	err = setupDirs([]string{testRootDir, testRootDir2})
	assert(t, (err == nil), err)

	for name, maskTest := range MaskTests {

		fmt.Printf("Running test for mask %q\n", name)
		err = maskTest.Setup(maskTest.Args...)
		assert(t, (err == nil), err)

		w, err = fsevents.NewWatcher(testRootDir, maskTest.Mask)
		assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
		assert(t, (err == nil), err)

		w.StartAll()

		err = maskTest.Action(maskTest.Args...)
		assert(t, (err == nil), err)

		event, err := w.ReadSingleEvent()
		assert(t, (err == nil), err)

		// Ensure the event and its data is consistent
		assert(t, (event != nil), fmt.Errorf("ReadSingleEvent should have returned a non-nil event"))
		assert(t, (event.Name != ""), fmt.Errorf("The Name field in the event should not be empty"))
		assert(t, (event.Path != ""), fmt.Errorf("The Event field in the event should not be empty"))

		assert(t, (w.GetEventCount() == 1), nil)
		assert(t, (fsevents.CheckMask(maskTest.Mask, event.RawEvent.Mask) == true),
			fmt.Errorf("Event returned invalid mask: Expected: %d Got: %d\n", maskTest.Mask, event.RawEvent.Mask))

		w.StopAll()
		w.RemoveDescriptor(testRootDir)
	}

	err = teardownDirs([]string{testRootDir, testRootDir2})
	assert(t, (err == nil), err)
}

func TestCustomMaskChecks(t *testing.T) {

	var events = map[string]*fsevents.FsEvent{
		"IsDirEvent": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.IsDir},
		},
		"IsDirChanged": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.DirChangedEvent},
		},
		"IsDirCreated": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.DirCreatedEvent},
		},
		"IsDirRemoved": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.DirRemovedEvent},
		},
		"IsFileCreated": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.FileCreatedEvent},
		},
		"IsFileRemoved": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.FileRemovedEvent},
		},
		"IsFileChanged": &fsevents.FsEvent{
			RawEvent: &unix.InotifyEvent{Mask: fsevents.FileChangedEvent},
		},
	}

	// Loop through FsEvent's methods and test ensure that each return true
	for methodName, event := range events {
		t.Log("Testing FsEvent method:", methodName)
		returnVal := reflect.ValueOf(event).MethodByName(methodName).Call(nil)
		assert(t, (returnVal[0].Bool() == true), fmt.Errorf("FsEvent method %q should have returned true", methodName))
	}

	rootDeletedEvent := &fsevents.FsEvent{
		Path:     testRootDir,
		RawEvent: &unix.InotifyEvent{Mask: fsevents.RootDelete},
	}
	rootMovedEvent := &fsevents.FsEvent{
		Path:     testRootDir,
		RawEvent: &unix.InotifyEvent{Mask: fsevents.RootMove},
	}

	dirMethodArgs := []reflect.Value{reflect.ValueOf(testRootDir)}

	t.Log("Testing FsEvent method: IsRootDeletion")
	rootDelVal := reflect.ValueOf(rootDeletedEvent).MethodByName("IsRootDeletion").Call(dirMethodArgs)
	assert(t, (rootDelVal[0].Bool() == true), fmt.Errorf("FsEvent method 'IsRootDeletion' should have returned true"))

	t.Log("Testing FsEvent method: IsRootMoved")
	rootMovVal := reflect.ValueOf(rootMovedEvent).MethodByName("IsRootMoved").Call(dirMethodArgs)
	assert(t, (rootMovVal[0].Bool() == true), fmt.Errorf("FsEvent method 'IsRootMoved' should have returned true"))
}

func TestNewWatcher(t *testing.T) {
	var err error
	err = setupDirs([]string{testRootDir})
	assert(t, (err == nil), err)

	w, err := fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	err = teardownDirs([]string{testRootDir})
	assert(t, (err == nil), err)
}

func TestStart(t *testing.T) {
	var err error
	err = setupDirs([]string{testRootDir})
	assert(t, (err == nil), err)

	w, err := fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	d := w.GetDescriptorByPath(w.RootPath)
	assert(t, (d != nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))

	d.Start()
	assert(t, (w.GetRunningDescriptors() == 1), fmt.Errorf("GetRunningDescriptor should have returned 1"))

	err = teardownDirs([]string{testRootDir})
	assert(t, (err == nil), err)
}

func TestAddDescriptor(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	err = setupDirs([]string{testRootDir})
	assert(t, (err == nil), err)

	w, err = fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	// AddDescriptor SHOULD return ErrDescNotCreated if we try to add a WatchDescriptor for a directory that does not exist
	d, err := w.AddDescriptor("not_there/", fsevents.AllEvents)
	assert(t, (d == nil), fmt.Errorf("AddDescriptor should have returned nil descriptor"))
	expectedErr := fmt.Errorf("%s: %s", fsevents.ErrDescNotCreated, "directory does not exist").Error()
	assert(t, (err.Error() == expectedErr), err)
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	// AddDescriptor SHOULD return a non-nil WatchDescriptor and a non-nil error if we add a WatchDescriptor for a directory
	// that exists on disk and does not already have a running watch
	d, err = w.AddDescriptor(testRootDir, fsevents.AllEvents)
	assert(t, (d != nil), fmt.Errorf("AddDescriptor should have returned non-nil descriptor"))
	assert(t, (err != fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))

	// AddDescriptor SHOULD NOT return error if we add a WatchDescriptor for a directory that exists and is not already watched
	d, err = w.AddDescriptor(testRootDir, fsevents.AllEvents)
	assert(t, (d == nil), fmt.Errorf("AddDescriptor should have returned nil descriptor"))
	assert(t, (err == fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))
	assert(t, (len(w.ListDescriptors()) == 2), fmt.Errorf("ListDescriptors should have returned 1"))

	err = teardownDirs([]string{testRootDir})
	assert(t, (err == nil), err)
}

func TestGetDescriptorByPath(t *testing.T) {
	var err error
	err = setupDirs([]string{testRootDir})
	assert(t, (err == nil), err)

	w, err := fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("ListDescriptors should have returned 1"))

	w.AddDescriptor(testRootDir, fsevents.AllEvents)

	// GetDescriptorByPath SHOULD return a non-nil descriptor object if the path for the descriptor exists
	there := w.GetDescriptorByPath(testRootDir)
	assert(t, (there != nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))

	// GetDescriptorByPath SHOULD NOT return a non-nil descriptor object if the path for the descriptor DOES NOT exist
	notThere := w.GetDescriptorByPath("not_there")
	assert(t, (notThere == nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))

	err = teardownDirs([]string{testRootDir})
	assert(t, (err == nil), err)
}

type fileCreatedHandler struct {
	Mask uint32
}

func (h *fileCreatedHandler) Handle(w *fsevents.Watcher, event *fsevents.FsEvent) error {
	fmt.Println("File created:", event.Path)
	return nil
}

// GetMask returns the inotify event mask this EventHandler handles
func (h *fileCreatedHandler) GetMask() uint32 {
	return h.Mask
}

func (h *fileCreatedHandler) Check(event *fsevents.FsEvent) bool {
	return event.IsFileCreated()
}

func TestUnregisterEventHandler(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	os.Mkdir(testRootDir, 0777)

	w, err = fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (err == nil), err)

	err = w.UnregisterEventHandler(fsevents.FileCreatedEvent)
	assert(t, (err != nil), err)

	err = w.RegisterEventHandler(&fileCreatedHandler{Mask: fsevents.FileCreatedEvent})
	assert(t, (err == nil), err)

	err = w.UnregisterEventHandler(fsevents.FileCreatedEvent)
	assert(t, (err == nil), err)

	os.RemoveAll(testRootDir)
}

func TestRegisterEventHandler(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	os.Mkdir(testRootDir, 0777)

	w, err = fsevents.NewWatcher(testRootDir, fsevents.AllEvents)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (err == nil), err)

	err = w.RegisterEventHandler(&fileCreatedHandler{Mask: fsevents.FileCreatedEvent})
	assert(t, (err == nil), err)

	err = w.RegisterEventHandler(&fileCreatedHandler{Mask: fsevents.FileCreatedEvent})
	assert(t, (err != nil), err)

	os.RemoveAll(testRootDir)
}

package fsevents_test

import (
	"fmt"
	"math/rand"
	"os"
	"path"
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

func setupDirs(paths []string) {
	for _, path := range paths {
		os.Mkdir(path, 0777)
	}
}

func teardownDirs(paths []string) {
	for _, path := range paths {
		os.RemoveAll(path)
	}
}

func TestMasks(t *testing.T) {
	var w *fsevents.Watcher
	var d *fsevents.WatchDescriptor
	var err error

	setupDirs([]string{testRootDir, testRootDir2})

	for name, maskTest := range MaskTests {

		fmt.Printf("Running test for mask %q\n", name)
		err = maskTest.Setup(maskTest.Args...)
		assert(t, (err == nil), err)

		w, err = fsevents.NewWatcher()
		assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
		assert(t, (err == nil), err)

		d, err = w.AddDescriptor(testRootDir, maskTest.Mask)
		assert(t, (d != nil), fmt.Errorf("AddDescriptor should have returned a non-nil descriptor"))
		assert(t, (err == nil), err)

		err = d.Start()
		assert(t, (err == nil), err)

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

	teardownDirs([]string{testRootDir, testRootDir2})
}

func TestRemoveWatchDescriptor(t *testing.T) {
	var err error
	setupDirs([]string{testRootDir})

	w, err := fsevents.NewWatcher()
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))
	assert(t, (err == nil), err)

	d, err := w.AddDescriptor(testRootDir, fsevents.AllEvents)
	assert(t, (d != nil), fmt.Errorf("AddDescriptor should have returned non-nil descriptor"))
	assert(t, (err != fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))

	// RemoveDescriptor SHOULD return non-nil when removing an existing descriptor
	err = w.RemoveDescriptor(d.Path)
	assert(t, (err == nil), fmt.Errorf("RemoveDescriptor should have returned nil error upon removal of existing descriptor"))

	teardownDirs([]string{testRootDir})
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

	// Loop through FsEvent's methods and test each to ensure that each return true
	for methodName, event := range events {
		t.Log("Running test for FsEvent method:", methodName)
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
	setupDirs([]string{testRootDir})

	w, err := fsevents.NewWatcher()
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))

	teardownDirs([]string{testRootDir})
}

func TestStart(t *testing.T) {
	var err error
	setupDirs([]string{testRootDir})

	w, err := fsevents.NewWatcher()
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned a non-nil Watcher"))

	d, err := w.AddDescriptor(testRootDir, fsevents.AllEvents)
	assert(t, (d != nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))
	assert(t, (err == nil), err)

	// Trying to start a new descriptor SHOULD NOT return error
	err = d.Start()
	assert(t, (err == nil), err)
	assert(t, (w.GetRunningDescriptors() == 1), fmt.Errorf("GetRunningDescriptor should have returned 1"))

	// Trying to start an already running descriptor SHOULD return error
	err = d.Start()
	assert(t, (err != nil), fmt.Errorf("Start should have returned error %q", fsevents.ErrDescNotStart))

	teardownDirs([]string{testRootDir})
}

func TestAddDescriptor(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	setupDirs([]string{testRootDir})

	w, err = fsevents.NewWatcher()
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))

	// AddDescriptor SHOULD return ErrDescNotCreated if we try to add a WatchDescriptor for a directory that does not exist
	d, err := w.AddDescriptor("not_there/", fsevents.AllEvents)
	assert(t, (d == nil), fmt.Errorf("AddDescriptor should have returned nil descriptor"))
	expectedErr := fmt.Errorf("%s: %s", fsevents.ErrDescNotCreated, "directory does not exist").Error()
	assert(t, (err.Error() == expectedErr), err)
	assert(t, (len(w.ListDescriptors()) == 0), fmt.Errorf("len(w.ListDescriptors()) should have returned 0"))

	// AddDescriptor SHOULD return a non-nil WatchDescriptor and a non-nil error if we add a WatchDescriptor for a directory
	// that exists on disk and does not already have a running watch
	d, err = w.AddDescriptor(testRootDir, fsevents.AllEvents)
	assert(t, (d != nil), fmt.Errorf("AddDescriptor should have returned non-nil descriptor"))
	assert(t, (err != fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("len(w.ListDescriptors()) should have returned 1"))

	// AddDescriptor SHOULD return error if we add a WatchDescriptor for a directory that exists and is not already watched
	d, err = w.AddDescriptor(testRootDir, fsevents.AllEvents)
	assert(t, (d == nil), fmt.Errorf("AddDescriptor should have returned nil descriptor"))
	assert(t, (err == fsevents.ErrDescAlreadyExists), fmt.Errorf("AddDescriptor should have returned error on duplicate descriptor"))
	assert(t, (len(w.ListDescriptors()) == 1), fmt.Errorf("len(w.ListDescriptors()) should have returned 1"))

	teardownDirs([]string{testRootDir})
}

func TestGetDescriptorByPath(t *testing.T) {
	var err error
	setupDirs([]string{testRootDir})

	w, err := fsevents.NewWatcher()
	assert(t, (err == nil), err)
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))

	w.AddDescriptor(testRootDir, fsevents.AllEvents)

	// GetDescriptorByPath SHOULD return a non-nil descriptor object if the path for the descriptor exists
	there := w.GetDescriptorByPath(testRootDir)
	assert(t, (there != nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))

	// GetDescriptorByPath SHOULD NOT return a non-nil descriptor object if the path for the descriptor DOES NOT exist
	notThere := w.GetDescriptorByPath("not_there")
	assert(t, (notThere == nil), fmt.Errorf("GetDescriptorByPath should have returned non-nil descriptor"))

	teardownDirs([]string{testRootDir})
}

type fileCreatedHandler struct {
	Mask uint32
}

func (h *fileCreatedHandler) Handle(w *fsevents.Watcher, event *fsevents.FsEvent) error {
	fmt.Println("File created:", event.Path)
	return nil
}

func (h *fileCreatedHandler) GetMask() uint32 {
	return h.Mask
}

func (h *fileCreatedHandler) Check(event *fsevents.FsEvent) bool {
	return event.IsFileCreated()
}

func TestUnregisterEventHandler(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	setupDirs([]string{testRootDir})

	w, err = fsevents.NewWatcher()
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (err == nil), err)

	// UnregisterEventHandler SHOULD return an error when attempting to remove a non-existant handle
	err = w.UnregisterEventHandler(fsevents.FileCreatedEvent)
	assert(t, (err != nil), err)

	// RegisterEventHandler SHOULD NOT return an error when attempting to register a non-existant handle
	err = w.RegisterEventHandler(&fileCreatedHandler{Mask: fsevents.FileCreatedEvent})
	assert(t, (err == nil), err)

	// RegisterEventHandler SHOULD return an error when attempting to register an existing handle
	err = w.RegisterEventHandler(&fileCreatedHandler{Mask: fsevents.FileChangedEvent})
	assert(t, (err == nil), err)

	err = w.UnregisterEventHandler(fsevents.FileCreatedEvent)
	assert(t, (err == nil), err)

	teardownDirs([]string{testRootDir})
}

func TestRegisterEventHandler(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	setupDirs([]string{testRootDir})

	w, err = fsevents.NewWatcher()
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (err == nil), err)

	handle := &fileCreatedHandler{Mask: fsevents.FileCreatedEvent}

	err = w.RegisterEventHandler(handle)
	assert(t, (err == nil), err)

	err = w.RegisterEventHandler(handle)
	assert(t, (err != nil), err)

	teardownDirs([]string{testRootDir})
}

func TestRecursiveAdd(t *testing.T) {
	var w *fsevents.Watcher
	var err error

	testDirs := []string{
		testRootDir,
		path.Join(testRootDir, "a"),
		path.Join(testRootDir, "a/aa"),
		path.Join(testRootDir, "b"),
		path.Join(testRootDir, "b/bb"),
	}

	setupDirs(testDirs)

	w, err = fsevents.NewWatcher()
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (err == nil), err)

	err = w.RecursiveAdd(testRootDir, fsevents.AllEvents)
	assert(t, (err == nil), err)
	assert(t, (int(w.GetRunningDescriptors()) == len(testDirs)), fmt.Errorf("Count of running descriptors is not equal to number of directories"))

	teardownDirs(testDirs)
}

func TestMultipleEvents(t *testing.T) {
	var w *fsevents.Watcher
	var d *fsevents.WatchDescriptor
	var err error

	testPath := testRootDir + "/multiple-event-test"

	setupDirs([]string{testRootDir})

	err = writeRandomFile(testPath)
	assert(t, (err == nil), err)

	w, err = fsevents.NewWatcher()
	assert(t, (w != nil), fmt.Errorf("NewWatcher should have returned non-nil Watcher"))
	assert(t, (err == nil), err)

	d, err = w.AddDescriptor(testPath, unix.IN_ALL_EVENTS)
	assert(t, (d != nil), fmt.Errorf("AddDescriptor should have returned a non-nil descriptor"))
	assert(t, (err == nil), err)

	err = d.Start()
	assert(t, (err == nil), err)

	err = os.Remove(testPath)
	assert(t, (err == nil), err)

	// As per inotify(7), an unlink triggers 3 separate events on the
	// unlinked path: IN_ATTRIB, IN_DELETE_SELF, and IN_IGNORED.  Since
	// we can't timeout ReadSingleEvent, run it in a goroutine and fail
	// the test if we don't read those 3 events in a reasonable amount
	// of time.
	eventCount := 0
	go func() {
		for {
			event, err := w.ReadSingleEvent()
			assert(t, (event != nil), err)
			assert(t, (err == nil), err)
			eventCount++
		}
	}()
	time.Sleep(1 * time.Millisecond)
	assert(t, (eventCount == 3), err)

	teardownDirs([]string{testRootDir})
}

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
	UnixMask int
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

func eq(compare bool, err error) {
	if compare == false {
		_, _, line, _ := runtime.Caller(1)
		if err != nil {
			fmt.Printf("Comparison @ line: %d false\n", line)
			fmt.Println("Error returned:", err)

			os.RemoveAll(testRootDir)
			os.RemoveAll(testRootDir2)

			os.Exit(-1)
		} else {
			fmt.Printf("Comparison @ line: %d false\n", line)
			os.RemoveAll(testRootDir)
			os.RemoveAll(testRootDir2)

			os.Exit(-1)
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

	testOptions := &fsevents.WatcherOptions{
		Recursive:       true,
		UseWatcherFlags: false,
	}

	var successfulTestCount int = 0
	for i, maskTest := range MaskTests {

		fmt.Printf("Running test %d: %s\n", i, maskTest.Action)
		fmt.Printf("Running setup function... ")
		err = maskTest.Setup(maskTest.Args...)
		eq((err == nil), err)

		w, err = fsevents.NewWatcher(testRootDir, maskTest.UnixMask, testOptions)
		eq((w != nil), nil)
		eq((err == nil), err)

		fmt.Printf("Running Watcher ... ")
		w.StartAll()

		fmt.Printf("Running test action ... ")
		testFunc := Actions[maskTest.Action]
		err = testFunc(maskTest.Args...)
		eq((err == nil), err)

		fmt.Printf("Reading event ... \n")
		event, err := w.ReadSingleEvent()
		eq((err == nil), err)

		// Ensure the event and its data is consistent
		eq((event != nil), nil)
		eq((event.Name != ""), nil)
		eq((event.Path != ""), nil)

		eq((w.GetEventCount() == 1), nil)
		eq((fsevents.CheckMask(maskTest.UnixMask, event.RawEvent.Mask) == true),
			fmt.Errorf("Event returned invalid mask: Expected: %d Got: %d\n", maskTest.UnixMask, event.RawEvent.Mask))

		w.StopAll()
		w.RemoveDescriptor(testRootDir)
		w = nil

		successfulTestCount++
		fmt.Printf("Test finished successfully\n")
		fmt.Printf("------------------------\n")
	}
	fmt.Printf("%d tests ran successfully\n", successfulTestCount)
}

func TestNewWatcher(t *testing.T) {

	setupTestDirectories(testRootDir)
	setupTestDirectories(testRootDir2)
	defer removeTestDirectories()

	testOptions := &fsevents.WatcherOptions{
		Recursive:       false,
		UseWatcherFlags: false,
	}
	w, err := fsevents.NewWatcher(testRootDir, fsevents.AllEvents, testOptions)
	eq((err == nil), err)
	eq((w != nil), nil)

	for _, d := range w.ListDescriptors() {
		fmt.Println(d)
	}
}

func TestAddDescriptor(t *testing.T) {

	setupTestDirectories(testRootDir)
	setupTestDirectories(testRootDir2)
	defer removeTestDirectories()

	testOptions := &fsevents.WatcherOptions{
		Recursive:       false,
		UseWatcherFlags: false,
	}
	var w *fsevents.Watcher
	var err error

	w, err = fsevents.NewWatcher(testRootDir, fsevents.AllEvents, testOptions)
	eq((w != nil), nil)
	eq((err == nil), err)

	// err should be != nil if we add watch that already exists
	err = w.AddDescriptor(testRootDir, -1)
	eq((err != fsevents.ErrDescAlreadyExists), err)

	// err should be != nil if directory does not exist
	err = w.AddDescriptor("not_there/", -1)
	expectedErr := fmt.Errorf("%s: %s", fsevents.ErrDescNotCreated, "directory does not exist").Error()
	eq((err.Error() == expectedErr), err)
}

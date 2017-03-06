package fsevents

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

type watchDescriptor struct {
	Path            string
	Mask            int
	WatchDescriptor int
}

type FsEvent struct {
	Path       string
	RawEvent   *unix.InotifyEvent
	Descriptor *watchDescriptor
}

type Watcher struct {
	sync.Mutex
	// The root path of this watcher
	RootPath string
	// The main inotify descriptor
	FileDescriptor int
	// Default masked is applied to watches in AddWatch if no inotify flags are specified
	DefaultMask int
	// Watch descriptors in this watch key: watch path -> value: watchDescriptor
	Descriptors map[string]*watchDescriptor
	// The event channel we send all events on
	Events chan *FsEvent
	// How we report errors
	Errors chan error
}

const (
	// Default inotify flags
	Accessed   = unix.IN_ACCESS
	Modified   = unix.IN_MODIFY
	AttrChange = unix.IN_ATTRIB
	CloseWrite = unix.IN_CLOSE_WRITE
	CloseRead  = unix.IN_CLOSE_NOWRITE
	Open       = unix.IN_OPEN
	MovedFrom  = unix.IN_MOVED_FROM
	MovedTo    = unix.IN_MOVED_TO
	Move       = unix.IN_MOVE
	Create     = unix.IN_CREATE
	Delete     = unix.IN_DELETE
	RootDelete = unix.IN_DELETE_SELF
	RootMove   = unix.IN_MOVE_SELF
	IsDir      = unix.IN_ISDIR
)

const (
	// fsevents flags

	// Use the default flags in the Watcher struct when adding a new watch
	UseWatcherFlags = iota
)

var (
	ErrWatchNotCreated    = errors.New("Watch descriptor could not be created")
	ErrWatchAlreadyExists = errors.New("Watch already exists")
	ErrWatchNotStart      = errors.New("Watch could not be started")
	ErrWatchNotStopped    = errors.New("Watch could not be stopped")
	ErrWatchNotRemoved    = errors.New("Watch could not be removed")
	ErrIncompleteRead     = errors.New("Incomplete even read")
	ErrReadError          = errors.New("There was an error reading an event")
	ErrDescriptorNotFound = errors.New("Descriptor for event not found")
)

func newWatchDescriptor(dirPath string, mask int) (*watchDescriptor, error) {
	return &watchDescriptor{
		Path:            dirPath,
		WatchDescriptor: -1,
		Mask:            mask,
	}, nil
}

//DescriptorExists returns true if a WatchDescriptor exists in w.Descriptors, false otherwise
func (w *Watcher) DescriptorExists(watchPath string) bool {
	if _, exists := w.Descriptors[watchPath]; exists {
		return true
	}
	return false
}

func (w *Watcher) AddDescriptor(dirPath string, flag int) error {
	if w.DescriptorExists(dirPath) == true {
		return ErrWatchAlreadyExists
	}
	var inotifymask int
	if flag == UseWatcherFlags {
		inotifymask = w.DefaultMask
	} else {
		inotifymask = flag
	}
	new, err := newWatchDescriptor(dirPath, inotifymask)
	if err != nil {
		return err
	}

	w.Lock()
	w.Descriptors[dirPath] = new
	w.Unlock()

	return nil
}

func (w *Watcher) RecursiveAdd(rootPath string, flag int) error {
	dirStat, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return err
	}
	for _, child := range dirStat {
		if child.IsDir() == true {
			w.AddDescriptor(child.Name(), flag)
		}
	}
	return nil
}

func NewWatcher(rootPath string, defaultMask int) (*Watcher, error) {
	// func InotifyInit() (fd int, err error)
	fd, err := unix.InotifyInit()
	if fd == -1 || err != nil {
		return nil, fmt.Errorf("%s: %s", ErrWatchNotCreated, err)
	}
	w := &Watcher{
		RootPath:       rootPath,
		FileDescriptor: fd,
		DefaultMask:    defaultMask,
		Descriptors:    make(map[string]*watchDescriptor),
		Events:         make(chan *FsEvent),
		Errors:         make(chan error),
	}
	return w, w.AddDescriptor(rootPath, defaultMask)
}

// Start() start starts a watchDescriptors inotify even watcher
func (d *watchDescriptor) start(fd int) error {
	var err error
	d.WatchDescriptor, err = unix.InotifyAddWatch(fd, d.Path, uint32(d.Mask))
	if d.WatchDescriptor == -1 {
		return ErrWatchNotStart
	}
	return err
}

//StartAll() Start all inotify watches described by this Watcher
func (w *Watcher) StartAll() error {
	w.Lock()
	defer w.Unlock()
	for _, d := range w.Descriptors {
		if err := d.start(w.FileDescriptor); err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) getWatchDescriptor(wd int) *watchDescriptor {
	w.Lock()
	defer w.Unlock()
	for _, d := range w.Descriptors {
		if d.WatchDescriptor == wd {
			return d
		}
	}
	return nil
}

// Most of this function was copied from https://github.com/fsnotify/fsnotify
// Cheers to the authors of this project.
func (w *Watcher) Watch() {
	var buffer [unix.SizeofInotifyEvent * 4096]byte
	for {
		bytesRead, err := unix.Read(w.FileDescriptor, buffer[:])
		if bytesRead < unix.SizeofInotifyEvent {
			w.Errors <- ErrIncompleteRead
			continue
		} else if bytesRead == -1 || err != nil {
			w.Errors <- fmt.Errorf("%s: %s", ErrReadError.Error(), err)
			continue
		}
		// Offset in the event data pointer - reset to 0 every loop
		var offset uint32
		// Pointer to the event
		var rawEvent *unix.InotifyEvent
		var descriptor *watchDescriptor
		for offset <= uint32(bytesRead-unix.SizeofInotifyEvent) {
			rawEvent = (*unix.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
			descriptor = w.getWatchDescriptor(int(rawEvent.Wd))

			if descriptor == nil {
				w.Errors <- ErrDescriptorNotFound
				continue
			}

			eventPath := descriptor.Path
			if rawEvent.Len > 0 {
				//Grab the even name and make it a path
				bytes := (*[unix.PathMax]byte)(unsafe.Pointer(&buffer[offset+unix.SizeofInotifyEvent]))
				eventPath += strings.TrimRight(string(bytes[0:rawEvent.Len]), "\000")
				eventPath = path.Clean(eventPath)
			}

			//Make our event and send if over the channel
			event := &FsEvent{
				Path:       eventPath,
				Descriptor: descriptor,
				RawEvent:   rawEvent,
			}
			w.Events <- event
			offset += (unix.SizeofInotifyEvent + rawEvent.Len)
		}
	}
}

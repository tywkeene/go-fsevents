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
	// The path of this descriptor
	Path string
	// This descriptor's inotify watch mask
	Mask int
	// This descriptor's inotify watch descriptor
	WatchDescriptor int
}

type FsEvent struct {
	// The path of the event
	Path string
	// The raw inotify event
	RawEvent *unix.InotifyEvent
	// The actual inotify watch descriptor related to this event
	Descriptor *watchDescriptor
}

type WatcherOptions struct {
	// Should this watcher be recursive?
	Recursive bool
	// Should we use the watcher's default inotify mask when creating new watchDescriptors?
	UseWatcherFlags bool
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
	// This watcher's options
	Options *WatcherOptions
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

var (
	// All the errors returned by fsevents
	// Should probably provide a more situationally descriptive message along with it
	ErrWatchNotCreated    = errors.New("watch descriptor could not be created")
	ErrWatchAlreadyExists = errors.New("watch already exists")
	ErrWatchNotStart      = errors.New("watch could not be started")
	ErrWatchNotStopped    = errors.New("watch could not be stopped")
	ErrWatchNotRemoved    = errors.New("watch could not be removed")
	ErrIncompleteRead     = errors.New("incomplete event read")
	ErrReadError          = errors.New("error reading an event")
	ErrDescriptorNotFound = errors.New("descriptor for event not found")
)

func newWatchDescriptor(dirPath string, mask int) *watchDescriptor {
	return &watchDescriptor{
		Path:            dirPath,
		WatchDescriptor: -1,
		Mask:            mask,
	}
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

// DescriptorExists returns true if a WatchDescriptor exists in w.Descriptors, false otherwise
func (w *Watcher) DescriptorExists(watchPath string) bool {
	if _, exists := w.Descriptors[watchPath]; exists {
		return true
	}
	return false
}

// AddDescriptor() will add a descriptor to Watcher w. The descriptor is not started
func (w *Watcher) AddDescriptor(dirPath string, mask int) error {
	if w.DescriptorExists(dirPath) == true {
		return ErrWatchAlreadyExists
	}
	var inotifymask int
	if w.Options.UseWatcherFlags == true {
		inotifymask = w.DefaultMask
	} else {
		inotifymask = mask
	}

	w.Lock()
	w.Descriptors[dirPath] = newWatchDescriptor(dirPath, inotifymask)
	w.Unlock()

	return nil
}

// Recursive add will add the directory at rootPath, and all directories below it, using the flags provided in mask
func (w *Watcher) RecursiveAdd(rootPath string, mask int) error {
	dirStat, err := ioutil.ReadDir(rootPath)
	if err != nil {
		return err
	}

	var inotifymask int
	if w.Options.UseWatcherFlags == true {
		inotifymask = w.DefaultMask
	} else {
		inotifymask = mask
	}

	for _, child := range dirStat {
		if child.IsDir() == true {
			childPath := path.Clean(path.Join(rootPath, child.Name()))
			if err := w.RecursiveAdd(childPath, inotifymask); err != nil {
				return err
			}
			if err := w.AddDescriptor(childPath, inotifymask); err != nil {
				return err
			}
		}
	}
	return nil
}

// NewWatcher allocates a new watcher at path rootPath, with the default mask defaultMask
// This function initializes inotify, so it must be run first
func NewWatcher(rootPath string, defaultMask int, options *WatcherOptions) (*Watcher, error) {
	// func InotifyInit() (fd int, err error)
	fd, err := unix.InotifyInit()
	if fd == -1 || err != nil {
		return nil, fmt.Errorf("%s: %s", ErrWatchNotCreated, err)
	}
	w := &Watcher{
		RootPath:       path.Clean(rootPath),
		FileDescriptor: fd,
		DefaultMask:    defaultMask,
		Descriptors:    make(map[string]*watchDescriptor),
		Events:         make(chan *FsEvent),
		Errors:         make(chan error),
		Options:        options,
	}
	if options.Recursive == true {
		if err := w.AddDescriptor(w.RootPath, defaultMask); err != nil {
			return w, err
		}
		return w, w.RecursiveAdd(w.RootPath, defaultMask)
	}
	return w, w.AddDescriptor(w.RootPath, defaultMask)
}

// StartAll() Start all inotify watches described by this Watcher
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

// getWatchDesccriptor() searches a Watcher instance for a watch descriptor.
// Searches by inotify watch descriptor
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
				// Grab the event name and make it a path
				bytes := (*[unix.PathMax]byte)(unsafe.Pointer(&buffer[offset+unix.SizeofInotifyEvent]))
				eventPath += "/" + strings.TrimRight(string(bytes[0:rawEvent.Len]), "\000")
				eventPath = path.Clean(eventPath)
			}

			// Make our event and send if over the channel
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

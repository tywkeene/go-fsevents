// Package fsevents provides routines for monitoring filesystem events
// on linux systems via inotify recursively.
package fsevents

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/unix"
)

type WatchDescriptor struct {
	// The path of this descriptor
	Path string
	// This descriptor's inotify watch mask
	Mask int
	// This descriptor's inotify watch descriptor
	WatchDescriptor int
	// Is this watcher currently running?
	Running bool
}

type FsEvent struct {
	// The name of the event's file
	Name string
	// The full path of the event
	Path string
	// The raw inotify event
	RawEvent *unix.InotifyEvent
	// The actual inotify watch descriptor related to this event
	Descriptor *WatchDescriptor
}

type WatcherOptions struct {
	// Should this watcher be recursive?
	Recursive bool
	// Should we use the watcher's default inotify mask when creating new WatchDescriptors?
	UseWatcherFlags bool
}

type Watcher struct {
	sync.Mutex
	// The root path of this watcher
	RootPath string
	// The main inotify descriptor
	FileDescriptor int
	// Default mask is applied to watches in AddWatch if no inotify flags are specified
	DefaultMask int
	// Watch descriptors in this watch key: watch path -> value: WatchDescriptor
	Descriptors map[string]*WatchDescriptor
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

	AllEvents = (Accessed | Modified | AttrChange | CloseWrite | CloseRead | Open | MovedFrom |
		MovedTo | MovedTo | Create | Delete | RootDelete | RootMove | IsDir)

	// Custom event flags

	// Directory events

	// A quick breakdown, same goes for the file events, except
	// those pertain to files, not directories. There is a difference.

	// The directory is not in the watch directory anymore
	// whether it was moved or deleted, it's *poof* gone
	DirRemovedEvent = MovedFrom | Delete | IsDir

	// Whether it was moved or copied into the watch directory,
	// or created with mkdir, there is a new directory
	DirCreatedEvent = MovedTo | Create | IsDir

	// A directory was closed with write permissions, modified, or its
	// attributes changed in some way
	DirChangedEvent = CloseWrite | Modified | AttrChange | IsDir

	// File events
	FileRemovedEvent = MovedFrom | Delete
	FileCreatedEvent = MovedTo | Create
	FileChangedEvent = CloseWrite | Modified | AttrChange

	// Root watch directory events
	RootEvent = RootDelete | RootMove
)

// CheckMask() returns true if flag 'check' is found in bitmask 'mask'
func CheckMask(check int, mask uint32) bool {
	return ((int(mask) & check) != 0)
}

// IsDirEvent() Returns true if the event is a directory event
func (e *FsEvent) IsDirEvent() bool {
	return CheckMask(IsDir, e.RawEvent.Mask)
}

// Root events.

// IsRootDeletion() returns true if the event contains the inotify flag IN_DELETE_SELF
// This means the root watch directory has been deleted,
// and there will be no more events read from the descriptor
// since it doesn't exist anymore. You should probably handle this
// gracefully and always check for this event before doing anything else
// Also be sure to add the RootDelete flag to your watched events when
// initializing fsevents
func (e *FsEvent) IsRootDeletion() bool {
	return CheckMask(RootDelete, e.RawEvent.Mask)
}

// IsRootDeletion() returns true if the event contains the inotify flag IN_MOVE_SELF
// This means the root watch directory has been moved. This may not matter
// to you at all, and depends on how you deal with paths in your program.
// Still, you should check for this event before doing anything else.
func (e *FsEvent) IsRootMoved() bool {
	return CheckMask(RootMove, e.RawEvent.Mask)
}

// Custom directory events

// Directory was closed with write permissions, modified, or its attributes changed
func (e *FsEvent) IsDirChanged() bool {
	return ((CheckMask(CloseWrite, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true)) ||
		((CheckMask(Modified, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true)) ||
		((CheckMask(AttrChange, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true))
}

// Directory created within the root watch, or moved into the root watch directory
func (e *FsEvent) IsDirCreated() bool {
	return ((CheckMask(Create, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true)) ||
		((CheckMask(MovedTo, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true))
}

// Directory deleted or moved out of the root watch directory
func (e *FsEvent) IsDirRemoved() bool {
	return ((CheckMask(Delete, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true)) ||
		((CheckMask(MovedFrom, e.RawEvent.Mask) == true) && (e.IsDirEvent() == true))
}

// Custom file events

// File was moved into, or created within the root watch directory
func (e *FsEvent) IsFileCreated() bool {
	return (((CheckMask(Create, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false)) ||
		((CheckMask(MovedTo, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false)))
}

// File was deleted or moved out of the root watch directory
func (e *FsEvent) IsFileRemoved() bool {
	return ((CheckMask(Delete, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false) ||
		((CheckMask(MovedFrom, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false)))
}

// File was closed with write permissions, modified, or its attributes changed
func (e *FsEvent) IsFileChanged() bool {
	return ((CheckMask(CloseWrite, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false)) ||
		((CheckMask(Modified, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false)) ||
		((CheckMask(AttrChange, e.RawEvent.Mask) == true) && (e.IsDirEvent() == false))
}

var (
	// All the errors returned by fsevents
	// Should probably provide a more situationally descriptive message along with it
	ErrWatchNotCreated     = errors.New("watch descriptor could not be created")
	ErrWatchAlreadyExists  = errors.New("watch already exists")
	ErrWatchNotExist       = errors.New("that watch does not exist")
	ErrWatchNotStart       = errors.New("watch could not be started")
	ErrWatchNotStopped     = errors.New("watch could not be stopped")
	ErrWatchNotRemoved     = errors.New("watch could not be removed")
	ErrIncompleteRead      = errors.New("incomplete event read")
	ErrReadError           = errors.New("error reading an event")
	ErrDescriptorNotFound  = errors.New("descriptor for event not found")
	ErrWatchNotRunning     = errors.New("descriptor is already stopped")
	ErrWatchAlreadyRunning = errors.New("descriptor is already running")
)

func newWatchDescriptor(dirPath string, mask int) *WatchDescriptor {
	return &WatchDescriptor{
		Path:            dirPath,
		WatchDescriptor: -1,
		Mask:            mask,
	}
}

// Start() start starts a WatchDescriptors inotify even watcher
func (d *WatchDescriptor) Start(fd int) error {
	var err error
	if d.Running == true {
		return ErrWatchAlreadyRunning
	}
	d.WatchDescriptor, err = unix.InotifyAddWatch(fd, d.Path, uint32(d.Mask))
	if d.WatchDescriptor == -1 || err != nil {
		d.Running = false
		return fmt.Errorf("%s: %s", ErrWatchNotStart, err)
	}
	d.Running = true
	return nil
}

// Stop() Stop a running watch descriptor
func (d *WatchDescriptor) Stop(fd int) error {
	if d.Running == false {
		return ErrWatchNotRunning
	}
	_, err := unix.InotifyRmWatch(fd, uint32(d.WatchDescriptor))
	if err != nil {
		return fmt.Errorf("%s: %s", ErrWatchNotStopped, err)
	}
	d.Running = false
	return nil
}

// DoesPathExist() Returns true if the path described by the descriptor exists
func (d *WatchDescriptor) DoesPathExist() bool {
	_, err := os.Lstat(d.Path)
	return os.IsExist(err)
}

// DescriptorExists() returns true if a WatchDescriptor exists in w.Descriptors, false otherwise
func (w *Watcher) DescriptorExists(watchPath string) bool {
	w.Lock()
	defer w.Unlock()
	if _, exists := w.Descriptors[watchPath]; exists {
		return true
	}
	return false
}

// ListDescriptors() Lists all currently active WatchDescriptors
func (w *Watcher) ListDescriptors() []string {
	list := make([]string, len(w.Descriptors))
	w.Lock()
	defer w.Unlock()
	for path, _ := range w.Descriptors {
		list = append(list, path)
	}
	return list
}

// RemoveDescriptor() removes the WatchDescriptor with the path matching path from the watcher,
// and stops the inotify watcher
func (w *Watcher) RemoveDescriptor(path string) error {
	if w.DescriptorExists(path) == false {
		return ErrWatchNotExist
	}
	w.Lock()
	defer w.Unlock()
	descriptor := w.Descriptors[path]
	if descriptor.DoesPathExist() == true {
		_, err := unix.InotifyRmWatch(w.FileDescriptor, uint32(descriptor.WatchDescriptor))
		if err != nil {
			return fmt.Errorf("%s: %s", ErrWatchNotStopped, err)
		}
	}
	delete(w.Descriptors, path)
	return nil
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

// RecursiveAdd() add the directory at rootPath, and all directories below it, using the flags provided in mask
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

// NewWatcher() allocate a new watcher at path rootPath, with the default mask defaultMask
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
		Descriptors:    make(map[string]*WatchDescriptor),
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
		if err := d.Start(w.FileDescriptor); err != nil {
			return err
		}
	}
	return nil
}

// StopAll() Stop all running watch descriptors
func (w *Watcher) StopAll() {
	w.Lock()
	defer w.Unlock()
	for _, d := range w.Descriptors {
		if d.Running == true {
			d.Stop(w.FileDescriptor)
		}
	}
}

// GetDesccriptorByWatch() searches a Watcher instance for a watch descriptor.
// Searches by inotify watch descriptor
func (w *Watcher) GetDescriptorByWatch(wd int) *WatchDescriptor {
	w.Lock()
	defer w.Unlock()
	for _, d := range w.Descriptors {
		if d.WatchDescriptor == wd {
			return d
		}
	}
	return nil
}

// GetDescriptorByPath() searches a Watcher instance for a watch descriptor.
// Searches by WatchDescriptor's path
func (w *Watcher) GetDescriptorByPath(watchPath string) *WatchDescriptor {
	if w.DescriptorExists(watchPath) == true {
		w.Lock()
		defer w.Unlock()
		return w.Descriptors[watchPath]
	}
	return nil
}

// Watch() Read events from inotify, and send them over the w.Events channel
// All errors are reported over the w.Errors channel
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
		var descriptor *WatchDescriptor
		for offset <= uint32(bytesRead-unix.SizeofInotifyEvent) {
			rawEvent = (*unix.InotifyEvent)(unsafe.Pointer(&buffer[offset]))
			descriptor = w.GetDescriptorByWatch(int(rawEvent.Wd))

			if descriptor == nil {
				w.Errors <- ErrDescriptorNotFound
				continue
			}

			var eventName string
			var eventPath string
			if rawEvent.Len > 0 {
				// Grab the event name and make the event path
				bytes := (*[unix.PathMax]byte)(unsafe.Pointer(&buffer[offset+unix.SizeofInotifyEvent]))
				eventName = strings.TrimRight(string(bytes[0:rawEvent.Len]), "\000")
				eventPath = path.Clean(path.Join(descriptor.Path, eventName))
			}

			// Make our event and send it over the channel
			event := &FsEvent{
				Name:       eventName,
				Path:       eventPath,
				Descriptor: descriptor,
				RawEvent:   rawEvent,
			}
			w.Events <- event
			offset += (unix.SizeofInotifyEvent + rawEvent.Len)
		}
	}
}

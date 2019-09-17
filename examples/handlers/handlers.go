package main

import (
	"log"
	"os"

	fsevents "github.com/tywkeene/go-fsevents"
)

// The DirectoryCreatedHandle implements the EventHandler interface
type DirectoryCreatedHandle struct {
	Mask uint32
}

// The Handle has access Watcher and Event this Handle was called from
// Returning an error from an EventHandler causes the Watcher to write the error to the Error channel
func (h *DirectoryCreatedHandle) Handle(w *fsevents.Watcher, event *fsevents.FsEvent) error {
	log.Println("Directory created:", event.Path)

	// The Watcher can be used inside event handles to add/remove/modify Watches
	// In this case, we add a descriptor for the created directory and start a watch for it
	d, err := w.AddDescriptor(event.Path, h.GetMask())
	if err != nil {
		return err
	}

	if err := d.Start(); err != nil {
		return err
	}
	log.Printf("Started watch on %q", event.Path)
	return nil
}

// GetMask returns the inotify event mask this EventHandler handles
func (h *DirectoryCreatedHandle) GetMask() uint32 {
	return h.Mask
}

// The most basic usage is to use the FsEvent methods to check the event mask against the handler
// Other logic may be used to determine if the handle should be executed for the given event.
func (h *DirectoryCreatedHandle) Check(event *fsevents.FsEvent) bool {
	return event.IsDirCreated()
}

func main() {
	if len(os.Args) == 1 {
		log.Fatalf("Must specify directory to watch\n")
	}

	var watchDir string = os.Args[1]
	var mask uint32 = fsevents.DirCreatedEvent

	w, err := fsevents.NewWatcher()
	if err != nil {
		panic(err)
	}

	_, err = w.AddDescriptor(watchDir, mask)
	if err != nil {
		panic(err)
	}

	// Register event handles
	w.RegisterEventHandler(&DirectoryCreatedHandle{Mask: fsevents.DirCreatedEvent})

	w.StartAll()

	// WatchAndHandle will search for the correct handle in response to a given
	// event and apply it, writing the error it returns, if any, to w.Errors
	go w.WatchAndHandle()

	for {
		select {
		case err := <-w.Errors:
			log.Println(err)
			break
		}
	}
}

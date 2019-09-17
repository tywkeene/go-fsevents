package main

import (
	"log"
	"os"

	fsevents "github.com/tywkeene/go-fsevents"
)

func handleEvents(w *fsevents.Watcher) error {

	// Watch for events
	go w.Watch()
	log.Println("Waiting for events...")

	for {
		select {
		case event := <-w.Events:
			// Contextual metadata is stored in the event object as well as a pointer to the WatchDescriptor that event belongs to
			log.Printf("Event Name: %s Event Path: %s Event Descriptor: %v", event.Name, event.Path, event.Descriptor)
			// A Watcher keeps a running atomic counter of all events it sees
			log.Println("Watcher Event Count:", w.GetEventCount())
			log.Println("Running descriptors:", w.GetRunningDescriptors())

			if event.IsDirCreated() == true {
				log.Println("Directory created:", event.Path)
				// A Watcher can be used dynamically in response to events to add/modify/delete WatchDescriptors
				d, err := w.AddDescriptor(event.Path, fsevents.DirCreatedEvent)
				if err != nil {
					log.Printf("Error adding descriptor for path %q: %s\n", event.Path, err)
					break
				}
				// WatchDescriptors can be started and stopped at any time and in response to events
				if err := d.Start(); err != nil {
					log.Printf("Error starting descriptor for path %q: %s\n", event.Path, err)
					break
				}
				log.Printf("Watch started for newly created directory %q\n", event.Path)
			}

			if event.IsDirRemoved() == true {
				log.Println("Directory removed:", event.Path)
				if err := w.RemoveDescriptor(event.Path); err != nil {
					log.Printf("Error removing descriptor for path %q: %s\n", event.Path, err)
					break
				}
			}

			if event.IsFileCreated() == true {
				log.Println("File created: ", event.Name)
			}
			if event.IsFileRemoved() == true {
				log.Println("File removed: ", event.Name)
			}
			break
		case err := <-w.Errors:
			log.Println(err)
			break
		}
	}
}

func main() {
	if len(os.Args) == 1 {
		panic("Must specify directory to watch")
	}
	watchDir := os.Args[1]
	// You might need to play with these flags to get the events you want
	// You can use these pre-defined flags that are declared in fsevents.go,
	// or the original inotify flags declared in the golang.org/x/sys/unix package

	var mask uint32 = fsevents.DirCreatedEvent | fsevents.DirRemovedEvent |
		fsevents.FileCreatedEvent | fsevents.FileRemovedEvent | fsevents.FileChangedEvent

	w, err := fsevents.NewWatcher()
	if err != nil {
		panic(err)
	}

	d, err := w.AddDescriptor(watchDir, mask)
	if err != nil {
		panic(err)
	}

	if err := d.Start(); err != nil {
		panic(err)
	}

	if err := handleEvents(w); err != nil {
		log.Fatalf("Error handling events: %s", err.Error())
	}
}

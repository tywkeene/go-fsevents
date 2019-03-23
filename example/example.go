package main

import (
	"log"
	"os"
	"path"

	fsevents "github.com/tywkeene/go-fsevents"
)

type DirectoryCreatedHandle struct {
	Mask uint32
}

func (d *DirectoryCreatedHandle) Handle(w *fsevents.Watcher, event *fsevents.FsEvent) error {
	log.Println("Handling directory create event ...")
	log.Println("Directory created:", path.Clean(event.Path))
	w.AddDescriptor(path.Clean(event.Path), 0)
	descriptor := w.GetDescriptorByPath(path.Clean(event.Path))
	descriptor.Start(w.FileDescriptor)
	return nil
}

func (d *DirectoryCreatedHandle) GetMask() uint32 {
	return d.Mask
}

func (d *DirectoryCreatedHandle) Check(event *fsevents.FsEvent) bool {
	return event.IsDirCreated()
}

func handleEvents(watcher *fsevents.Watcher) {
	watcher.RegisterEventHandle(fsevents.DirCreatedEvent, &DirectoryCreatedHandle{Mask: fsevents.DirCreatedEvent})

	watcher.StartAll()
	go watcher.WatchAndHandle()
	log.Println("Waiting for events...")
	for {
		select {
		case event := <-watcher.Events:
			log.Printf("Event Name: %s Event Path: %s Event Descriptor: %v", event.Name, event.Path, event.Descriptor)

			if event.IsDirRemoved() == true {
				log.Println("Directory removed:", path.Clean(event.Path))
				descriptor := watcher.GetDescriptorByWatch(int(event.RawEvent.Wd))
				if descriptor == nil {
					panic("GetDescriptorByPath() returned nil descriptor")
				}
				descriptor.Stop(descriptor.WatchDescriptor)
				watcher.RemoveDescriptor(path.Clean(event.Path))
			}

			if event.IsRootDeletion(watcher.RootPath) == true {
				log.Println("Root directory deleted")
				os.Exit(-1)
			}
			break
		case err := <-watcher.Errors:
			log.Println(err)
			break
		}
		log.Println("Descriptors", watcher.ListDescriptors())
		log.Println("Watcher Event Count:", watcher.GetEventCount())
		log.Println("Running descriptors:", watcher.GetRunningDescriptors())
		log.Printf("--------------\n\n")
	}
}

func main() {
	if len(os.Args) == 1 {
		panic("Must specify directory to watch")
	}
	watchDir := os.Args[1]

	options := &fsevents.WatcherOptions{
		// Recursive flag will make a watcher recursive,
		// meaning it will go all the way down the directory
		// tree, and add descriptors for all directories it finds
		Recursive: true,
		// UseWatcherFlags will use the flag passed to NewWatcher()
		// for all subsequently created watch descriptors
		UseWatcherFlags: true,
	}

	// You might need to play with these flags to get the events you want
	// You can use these pre-defined flags that are declared in fsevents.go,
	// or the original inotify flags declared in the golang.org/x/sys/unix package

	inotifyFlags := fsevents.DirCreatedEvent | fsevents.DirRemovedEvent |
		fsevents.FileCreatedEvent | fsevents.FileRemovedEvent |
		fsevents.FileChangedEvent | fsevents.RootEvent

	w, err := fsevents.NewWatcher(watchDir, inotifyFlags, options)
	if err != nil {
		panic(err)
	}
	handleEvents(w)
}

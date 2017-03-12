package main

import (
	"log"
	"os"
	"path"

	"github.com/tywkeene/go-fsevents"
)

func handleEvents(watcher *fsevents.Watcher) {
	watcher.StartAll()
	go watcher.Watch()
	log.Println("Waiting for events...")
	for {
		list := watcher.ListDescriptors()
		log.Println(list)
		select {
		case event := <-watcher.Events:
			log.Printf("Event Name: %s Event Path: %s", event.Name, event.Path)

			if event.IsDirCreated() == true {
				log.Println("Directory created:", path.Clean(event.Path))
				watcher.AddDescriptor(path.Clean(event.Path), 0)
			}
			if event.IsDirRemoved() == true {
				log.Println("Directory removed:", path.Clean(event.Path))
				watcher.RemoveDescriptor(path.Clean(event.Path))
			}

			if event.IsFileCreated() == true {
				log.Println("File created: ", event.Name)
			}
			if event.IsFileRemoved() == true {
				log.Println("File removed: ", event.Name)
			}
			break
		case err := <-watcher.Errors:
			log.Println(err)
			break
		}
	}
}

func main() {
	if len(os.Args) == 0 {
		panic("Must specify directory to watch")
	}
	watchDir := os.Args[1]

	options := &fsevents.WatcherOptions{
		Recursive:       true,
		UseWatcherFlags: true,
	}
	inotifyFlags := fsevents.Delete | fsevents.Create | fsevents.IsDir | fsevents.Modified | fsevents.MovedTo |
		fsevents.Modified
	w, err := fsevents.NewWatcher(watchDir, inotifyFlags, options)
	if err != nil {
		panic(err)
	}
	handleEvents(w)
}

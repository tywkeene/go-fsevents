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
		select {
		case event := <-watcher.Events:
			if (event.RawEvent.Mask&fsevents.Delete) == fsevents.Delete &&
				(event.RawEvent.Mask&fsevents.IsDir) == fsevents.IsDir {
				log.Println("Directory deleted:", path.Clean(event.Path))
			}
			if (event.RawEvent.Mask&fsevents.Create) == fsevents.Create &&
				(event.RawEvent.Mask&fsevents.IsDir) == fsevents.IsDir {
				log.Println("Directory created:", path.Clean(event.Path))
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
	inotifyFlags := fsevents.Delete | fsevents.Create | fsevents.IsDir
	w, err := fsevents.NewWatcher(watchDir, inotifyFlags, options)
	if err != nil {
		panic(err)
	}
	handleEvents(w)
}

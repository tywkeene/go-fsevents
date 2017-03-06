package main

import (
	"log"

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
				log.Println("Directory deleted:", event.Path)
			}
			if (event.RawEvent.Mask&fsevents.Create) == fsevents.Create &&
				(event.RawEvent.Mask&fsevents.IsDir) == fsevents.IsDir {
				log.Println("Directory created:", event.Path)
			}
			break
		case err := <-watcher.Errors:
			log.Println(err)
			break
		}
	}
}

func main() {
	w, err := fsevents.NewWatcher("/home/null/tmp/", fsevents.Delete|fsevents.Create|fsevents.IsDir)
	if err != nil {
		panic(err)
	}
	w.AddDescriptor("/home/null/var/", fsevents.UseWatcherFlags)
	handleEvents(w)
}

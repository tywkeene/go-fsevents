# go-fsevents
Recursive filesystem event watcher using inotify in golang

go-fsevents provides functions necessary for monitoring filesystem events on linux systems using the inotify interface.

Unlike other inotify packages, go-fsevents provides a recursive watcher, allowing the monitoring of directory trees easily.

## Quickstart

```
package main

import (
	"log"
	"os"
	"path"

	"github.com/tywkeene/go-fsevents"
)

func handleEvents(watcher *fsevents.Watcher) {

  // This will start all watch descriptors
	watcher.StartAll()
  // Now our watcher can receive events and send them over the channel
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
  
  // We're going to make a watcher that is recursive, and uses the default
  // inotify mask on any subsequent descriptors added to this watcher
	options := &fsevents.WatcherOptions{
		Recursive:       true,
		UseWatcherFlags: true,
	}
  // Watch for directory creation and deletion
	inotifyFlags := fsevents.Delete | fsevents.Create | fsevents.IsDir
  // Make our watcher
	w, err := fsevents.NewWatcher(watchDir, inotifyFlags, options)
	if err != nil {
		panic(err)
	}
	handleEvents(w)
}
```

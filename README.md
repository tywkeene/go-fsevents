# go-fsevents
Recursive filesystem event watcher using inotify in golang

go-fsevents provides functions necessary for monitoring filesystem events on linux systems using the inotify interface.

Unlike other inotify packages, go-fsevents provides a recursive watcher, allowing the monitoring of directory trees easily.

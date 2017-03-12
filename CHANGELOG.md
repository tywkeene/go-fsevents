## Mon 06 Mar 2017 04:15:27 PM MST Version: 0.0.1
Initial commit

Multiple directory even monitoring works

## Wed 08 Mar 2017 04:44:43 PM MST Version: 0.0.2
Comments and code cleanup

## Fri 10 Mar 2017 06:39:59 PM MST Version: 0.0.3
Make RecursiveAdd() actually work

Refactor and cleanup

## Fri 10 Mar 2017 08:29:10 PM MST Version: 0.0.4
Add ListDescriptors() and RemoveDescriptor()

Update example to showcase the proper use of these functions

## Fri 10 Mar 2017 09:06:36 PM MST Version: 0.0.5
Forgot to add mutex locks in ListDescriptors() and RemoveDescriptor()

## Sat 11 Mar 2017 01:31:32 PM MST Version: 0.0.6
Forgot to add mutex lock to DescriptorExists()

Fix comment typo

## Sat 11 Mar 2017 03:34:30 PM MST Version: 0.0.7
Rename getWatchDescriptor() to GetDescriptorByWatch()

## Sat 11 Mar 2017 03:37:41 PM MST Version: 0.0.8
Added GetDescriptorByPath()

## Sat 11 Mar 2017 03:49:17 PM MST Version: 0.0.9

Added Stop() for stopping running watch descriptors

Added d.Running to check a descriptor's status

Added status checks to d.Stop() and d.Start()

## Sat 11 Mar 2017 04:08:11 PM MST Version: 0.0.10
Added w.StopAll() to stop all currently running WatchDescriptors

## Sun 12 Mar 2017 03:02:50 PM MDT Version: 0.0.11

Added WatchDescriptor.DoesPathExist() that returns true if a descriptor's path
exists, false otherwise

Fixed Watcher.RemoveDescriptor() to not try to remove an inotify watch
of a file that has already been deleted, since inotify removes watches
itself. So we just need to handle our own bookkeeping.

## Sun 12 Mar 2017 03:17:26 PM MDT Version: 0.0.12

Refactor GetDescriptorByPath() to be a little less dumb

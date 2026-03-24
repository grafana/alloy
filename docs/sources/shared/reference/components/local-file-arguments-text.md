---
canonical: https://grafana.com/docs/alloy/latest/shared/reference/components/local-file-arguments-text/
description: Shared content, local file arguments text
headless: true
---

### File change detectors

File change detectors detect when the file needs to be re-read from disk. `local.file` supports two detectors: `fsnotify` and `poll`.

#### `fsnotify`

The `fsnotify` detector subscribes to filesystem events, which indicate when the watched file is updated.
This detector requires a filesystem that supports events at the operating system level. Network-based filesystems like NFS or FUSE won't work.

The component re-reads the watched file when a filesystem event is received.
This re-read happens for any filesystem event related to the file, including a permissions change.

`fsnotify` also polls for changes to the file with the configured `poll_frequency` as a fallback.

`fsnotify` stops receiving filesystem events if the watched file has been deleted, renamed, or moved.
The subscription is re-established on the next poll once the watched file exists again.

#### `poll`

The `poll` file change detector causes the watched file to be re-read every `poll_frequency`, regardless of whether the file changed.

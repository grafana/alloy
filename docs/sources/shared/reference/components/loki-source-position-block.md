The `position` block configures how a component tracks and persists read offsets.

| Name          | Type       | Description                                   | Default            | Required |
|---------------|------------|-----------------------------------------------|--------------------|----------|
| `key_mode`    | `string`   | How position entries are keyed.               | `"include_labels"` | no       |
| `sync_period` | `duration` | How often to sync the positions file to disk. | `"10s"`            | no       |

The `key_mode` argument must be one of:

* `"include_labels"`: Track positions by key and labels
* `"exclude_labels"`: Track positions by key only

When switching `key_mode`, existing positions remain readable through compatibility fallback lookups.

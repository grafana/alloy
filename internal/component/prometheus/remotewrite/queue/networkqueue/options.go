package networkqueue

import "time"

// QueueOptions handles the low level queue config options for a remote_write
type QueueOptions struct {
	Capacity          int           `alloy:"capacity,attr,optional"`
	MaxShards         int           `alloy:"max_shards,attr,optional"`
	MinShards         int           `alloy:"min_shards,attr,optional"`
	MaxSamplesPerSend int           `alloy:"max_samples_per_send,attr,optional"`
	BatchSendDeadline time.Duration `alloy:"batch_send_deadline,attr,optional"`
	MinBackoff        time.Duration `alloy:"min_backoff,attr,optional"`
	MaxBackoff        time.Duration `alloy:"max_backoff,attr,optional"`
	RetryOnHTTP429    bool          `alloy:"retry_on_http_429,attr,optional"`
	SampleAgeLimit    time.Duration `alloy:"sample_age_limit,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *QueueOptions) SetToDefault() {
	*r = DefaultQueueOptions
}

var DefaultQueueOptions = QueueOptions{
	Capacity:          10_000,
	MaxShards:         50,
	MinShards:         1,
	MaxSamplesPerSend: 1_000,
	BatchSendDeadline: 5 * time.Second,
	MinBackoff:        30 * time.Millisecond,
	MaxBackoff:        5 * time.Second,
	RetryOnHTTP429:    true,
}

# Queue based remote write client

# Caveat: Consider this the most experimental possible

## Overview

The `prometheus.write.queue` goals are to set reliable and repeatable memory and cpu usage based on the number of incoming and outgoing series. There are four broad parts to the system.

1. The `prometheus.write.queue` component itself. This handles the lifecycle of the Alloy system.
2. The `serialization` converts an array of series into a serializable format. This is handled via [msgp]() library. 
3. The `filequeue` is where the buffers are written to. This has a series of files that are committed to disk and then are read.
4. The `network` handles sending data. The data is sharded by the label hash across any number of loops that send data. The network layer supports HTTP proxy configuration and custom headers for increased flexibility.

Flow

appender -> serialization -> filequeue -> endpoint -> network

## Design Goals

The initial goal is to get a v1 version that will not include many features found in the existing remote write. This includes TLS specific options, scaling the network up, and any other features not found. Some of these features will be added over time, some will not.

## Major Parts

### actors

Underlying each of these major parts is an actor framework. The actor framework provides an work loop in the form of the func `DoWork`, each part is single threaded and only exposes a a handful of functions for sending and receiving data. Telemetry, configuration and other types of data are passed in via the work loop and handled one at a time. There are some allowances for setting atomic variables for specific scenarios. In the case of network retries it is necessary to break out of the tight loop. 

This means that the parts are inherently context free and single threaded which greatly simplifies the design. Communication is handled via [mailboxes] that are backed by channels underneath. By default these are asynchronous calls to an unbounded queue. Where that differs will be noted. 

Using actors, mailboxes and messages creates a system that responds to actions instead of polling or calling functions from other threads. This allows us to handle bounded queues easily for if the network is slow or down the `network` queue is bounded and will block on anyone trying to send more work.

The actual actor framework is never publicly exposed so that callers have no idea of what is running underneath.

In general each actor exposes one to many `Send` function(s), `Start` and `Stop`. 

### serialization

The `serialization` system provides a `prometheus.Appender` interface that is the entrance into the combined system. Each append function encodes the data into a serailization object `TimeSeriesBinary`, this represents a single prometheus signal. Above this is a `SeriesGroup` that contains slices for series and for metadata. Having a separate metadata set is optimal since metadata inherently behaves differently than normal series. Important note about `TimeSeriesBinary` is that it should always be created by a `sync.Pool` via `types.GetTimeSeriesBinary` and always returned to the pool via `types.PutTimeSeriesBinary`. This is a heavily used object and reuse is incredibly important to reduce garbage collection.

When each append is called it sends data to the `serializer` that adds to its `SeriesGroup`, the `serializer` can be shared among many appenders. There is one `serializer` for each endpoint. The `serializer` adds the the `TimeSeriesBinary` to an internal `SeriesGroup` and performs `FillBinary` that converts the standard labels to the deduplicated strings array. Filling in `LabelNames []int32` and `LabelValues []int32`. Once the threshold for maximum batch size is reached then the `serializer` will marshal the `SeriesGroup` to a byte slice. Create the appropriate metadata: version of the file format, series count, metadata count, strings count, and compression format. This will allow for future formats to be handled gracefully.

### filequeue

The `filequeue` handles writing and reading data from the `wal` directory. There exists one `filequeue` for each `endpoint` defined. Each file is represented by an atomicly increasing integer that is used to create a file named `<ID>.committed`. The committed name is simply to differentiate it from other files that may get created in the same directory. 

The `filequeue` accepts data `[]byte` and metadata `map[string]string`. These are also written using `msgp` for convenience. The `filequeue` keeps an internal array of files in order by id and fill feed them one by one to the `endpoint`, On startup the `filequeue` will load any existing files into the internal array and start feeding them to `endpoint`. When passing a handle to `endpoint` it passes a callback that actually returns the data and metadata. Once the callback is called then the file is deleted. It should be noted that this is done without touching any state within `filequeue`, keeping the zero mutex promise. It is assumed when the callback is called the data is being processed.

This does mean that the system is not ACID compliant. If a restart happens before memory is written or while it is in the sending queue it will be lost. This is done for performance and simplicity reasons.

### endpoint

The `endpoint` handles uncompressing the data, unmarshalling it to a `SeriesGroup` and feeding it to the `network` section. The `endpoint` is the parent of all the other parts and represents a single endpoint to write to. It ultimately controls the lifecycle of each child. 

### network

The `network` consists of two major sections, `manager` and `loop`. Inspired by the prometheus remote write the signals are placed in a queue by the label hash. This ensures that an out of order sample does not occur within a single instance and provides parrallelism. The `manager` handles picking which `loop` to send the data to and responding to configuration changes to change the configuration of a set of `loops`.

The `loop` is responsible for converting a set of `TimeSeriesBinary` to bytes and sending the data and responding. Due to the nature of the tight retry loop, it has an atomic bool to allow a stop value to be set and break out of the retry loop. The `loop` also provides stats, it should be noted these stats are not prometheus or opentelemetry, they are a callback for when stats are updated. This allows the caller to determine how to present the stats. The only requirement is that the callback be threadsafe to the caller.

The network layer now supports:
- HTTP proxy configuration (`proxy_url` parameter)
- Environment-based proxy detection (`proxy_from_environment` parameter)
- Custom HTTP headers for the main requests (`headers` parameter)
- Custom HTTP headers for proxy CONNECT requests (`proxy_connect_headers` parameter)

These features enhance the component's ability to work in enterprise environments with complex networking requirements and security configurations.  

### component

At the top level there is a standard component that is responsible for spinning up `endpoints` and passing configuration down.

## Implementation Goals

In normal operation memory should be limited to the scrape, memory waiting to be written to the file queue and memory in the queue to write to the network. This means that memory should not fluctuate based on the number of metrics written to disk and should be consistent.

Replayability, series will be replayed in the event of network downtime, or Alloy restart. Series TTL will be checked on writing to the `filequeue` and on sending to `network`.

### Consistency

Given a certain set of scrapes, the memory usage should be fairly consistent. Once written to disk no reference needs to be made to series. Only incoming and outgoing series contribute to memory. This does mean extreme care is taken to reduce allocations and by extension reduce garbage collection.

### Tradeoffs

In any given system there are tradeoffs, this system goal is to have a consistent memory footprint, reasonable disk reads/writes, and allow replayability. That comes with increased CPU cost, this can range anywhere from 25% to 50% more CPU. 

### Metrics backwards compatibility

Where possible metrics have been created to allow similiar dashboards to be used, with some caveats. The labels are slightly different, and there is no active series metric. Having an active series metric count would require knowing and storing a reference to every single unique series on disk. This would violate the core consistency goal.  

package queue

import (
	"context"
	"strconv"
	"time"

	snappy "github.com/eapache/go-xerial-snappy"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/vladopajic/go-actor/actor"
)

var _ actor.Worker = (*endpoint)(nil)

// endpoint handles communication between the serializer, filequeue and network.
type endpoint struct {
	network    types.NetworkClient
	serializer types.Serializer
	log        log.Logger
	ttl        time.Duration
	incoming   actor.Mailbox[types.DataHandle]
	buf        []byte
	self       actor.Actor
}

func NewEndpoint(client types.NetworkClient, serializer types.Serializer, ttl time.Duration, logger log.Logger) *endpoint {
	return &endpoint{
		network:    client,
		serializer: serializer,
		log:        logger,
		ttl:        ttl,
		incoming:   actor.NewMailbox[types.DataHandle](actor.OptCapacity(1)),
		buf:        make([]byte, 0, 1024),
	}
}

func (ep *endpoint) Start() {
	ep.self = actor.Combine(actor.New(ep), ep.incoming).Build()
	ep.self.Start()
	ep.serializer.Start()
	ep.network.Start()
}

func (ep *endpoint) Stop() {
	// Stop in order of data flow. This prevents errors around stopped mailboxes that can pop up.
	ep.serializer.Stop()
	ep.network.Stop()
	ep.network.Stop()
	ep.self.Stop()
}

func (ep *endpoint) DoWork(ctx actor.Context) actor.WorkerStatus {
	select {
	case <-ctx.Done():
		return actor.WorkerEnd
	case file, ok := <-ep.incoming.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		meta, buf, err := file.Pop()
		if err != nil {
			level.Error(ep.log).Log("msg", "unable to get file contents", "name", file.Name, "err", err)
			return actor.WorkerContinue
		}
		ep.deserializeAndSend(ctx, meta, buf)
		return actor.WorkerContinue
	}
}

func (ep *endpoint) deserializeAndSend(ctx context.Context, meta map[string]string, buf []byte) {
	var err error
	ep.buf, err = snappy.DecodeInto(ep.buf, buf)
	if err != nil {
		level.Debug(ep.log).Log("msg", "error snappy decoding", "err", err)
		return
	}
	// The version of each file is in the metadata. Right now there is only one version
	// supported but in the future the ability to support more. Along with different
	// compression.
	version, ok := meta["version"]
	if !ok {
		level.Error(ep.log).Log("msg", "version not found for deserialization")
		return
	}
	if version != "alloy.metrics.queue.v1" {
		level.Error(ep.log).Log("msg", "invalid version found for deserialization", "version", version)
		return
	}
	// Grab the amounts of each type and we can go ahead and alloc the space.
	seriesCount, _ := strconv.Atoi(meta["series_count"])
	metaCount, _ := strconv.Atoi(meta["meta_count"])
	stringsCount, _ := strconv.Atoi(meta["strings_count"])
	sg := &types.SeriesGroup{
		Series:   make([]*types.TimeSeriesBinary, seriesCount),
		Metadata: make([]*types.TimeSeriesBinary, metaCount),
		Strings:  make([]string, stringsCount),
	}
	// Prefill our series with items from the pool to limit allocs.
	for i := 0; i < seriesCount; i++ {
		sg.Series[i] = types.GetTimeSeriesFromPool()
	}
	for i := 0; i < metaCount; i++ {
		sg.Metadata[i] = types.GetTimeSeriesFromPool()
	}
	sg, ep.buf, err = types.DeserializeToSeriesGroup(sg, ep.buf)
	if err != nil {
		level.Debug(ep.log).Log("msg", "error deserializing", "err", err)
		return
	}

	for _, series := range sg.Series {
		// One last chance to check the TTL. Writing to the filequeue will check it but
		// in a situation where the network is down and writing backs up we dont want to send
		// data that will get rejected.
		seriesAge := time.Since(time.Unix(series.TS, 0))
		if seriesAge > ep.ttl {
			// TODO @mattdurham add metric here for ttl expired.
			continue
		}
		sendErr := ep.network.SendSeries(ctx, series)
		if sendErr != nil {
			level.Error(ep.log).Log("msg", "error sending to write client", "err", sendErr)
		}
	}

	for _, md := range sg.Metadata {
		sendErr := ep.network.SendMetadata(ctx, md)
		if sendErr != nil {
			level.Error(ep.log).Log("msg", "error sending metadata to write client", "err", sendErr)
		}
	}
}

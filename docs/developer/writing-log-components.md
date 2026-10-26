# Write log-forwarding components

Components that send log entries to other components have to survive config
reloads without deadlocking or losing data. This guide explains why that's hard,
the rules that make it work, and how to follow them with `loki.SourceRunner` and
`loki.Fanout`.

## How a reload runs

1. The loader evaluates every component, dependencies first. It builds new
   components and calls `Update` on changed ones.
1. The scheduler stops removed components and waits for their `Run` to return.
   Within each connected pipeline it stops them one at a time.
1. Only then does the scheduler start new components, or once the task shutdown
   deadline has passed.

`Update` is also called at any time when a component's dependency changes its
exports, for example when discovery targets change.

So while `Update` runs, a component that the new arguments send data to may not
be reading its input yet. It starts only after every `Update` in the reload has
returned and every removed component has stopped. Any downstream component can
be blocked, directly or through others, on such a component.

## The rules

The rules are on `component.Component` in
[`internal/component/component.go`](../../internal/component/component.go):

- `Update` must not wait on the data path. It must not wait for entries to be
  delivered or drained, for a producer that might be blocked sending, or on a
  lock that is held while sending.
- `Update` must switch to the downstream components in the new arguments before
  it returns, even when it returns an error. Removed components are stopped
  either way.
- Work that can block or fail, like stopping and starting producers or binding
  ports, belongs in `Run`. `Update` stores the desired config and signals `Run`,
  which retries failures and reports them through `HealthComponent`.
- Config that can never work must fail the reload. Check it in
  `Arguments.Validate`, not in `Run`, where it would only be retried.
- `Run` must return in bounded time once its context is canceled. New
  components only start after removed ones have returned from `Run`.

If the new arguments can't be evaluated, `Update` isn't called and the component
keeps its old downstream components.

Each rule is backed by a scenario in the [simulation](#test-with-the-simulation)
that fails without it.

## Sources: use `loki.SourceRunner`

`loki.SourceRunner` implements the rules for sources. A component supplies a
`Start` function that starts a producer for a config and returns a
`loki.Producer`: a `Stop` function, plus optional `Reconfigure` and `Exited`.
The runner does the rest:

```go
type Component struct {
	runner *loki.SourceRunner[producerConfig]
}

func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{}
	c.runner = loki.NewSourceRunner(loki.SourceRunnerOptions[producerConfig]{
		Logger: o.Logger,
		Start: func(ctx context.Context, cfg producerConfig) (loki.Producer[producerConfig], error) {
			// Bind ports, connect, start goroutines. Send entries to
			// c.runner.Entries() or c.runner.Batches().
			p, err := newProducer(cfg, c.runner.Entries())
			if err != nil {
				return loki.Producer[producerConfig]{}, err
			}
			return loki.Producer[producerConfig]{Stop: p.Stop}, nil
		},
	})
	return c, c.Update(args)
}

func (c *Component) Run(ctx context.Context) error {
	c.runner.Run(ctx)
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.runner.Update(newArgs.ForwardTo, newArgs.producerConfig())
	return nil
}

func (c *Component) CurrentHealth() component.Health {
	return c.runner.CurrentHealth()
}
```

What the runner does:

- `Update` switches the downstream components, stores the config and returns.
- `Run` starts the producer, and restarts it when the config changes. It always
  stops the old producer before it starts the new one, so the new one can take
  over ports, positions and consumer group memberships.
- Entries are forwarded on their own goroutine. Stopping a producer that waits
  for its entries to be handed off can't deadlock.
- A failed `Start` is retried with backoff and reported as unhealthy. Calls to
  `Update` with the same config wait for the scheduled retry.
- A producer that sets `Exited` can stop on its own, for example after an error
  it can't recover from. The runner reports it as unhealthy and starts a new
  producer with backoff.
- On shutdown, `Run` keeps forwarding while the producer stops, for at most
  `DrainTimeout`, and then drops what's left. An entry counts as accepted once
  the producer hands it to `Entries` or `Batches`, so a source that tracks
  positions loses what's dropped. When a reload removes several sources that
  can't drain, the reload can take up to one `DrainTimeout` per source.

What the component must do:

- Pass a config without the downstream components in it, so a change of
  `forward_to` alone doesn't restart the producer. If comparing configs with
  `reflect.DeepEqual` isn't right, set `Equal`.
- Make `Start` release everything it acquired when it fails.
- Make `Stop` return once the producer won't send any more entries. `Stop` may
  block while it hands off entries it holds. The runner calls it once per
  producer.
- Validate config in `Arguments.Validate`, so `Update` doesn't need to return
  errors. A config that only fails when the producer starts makes the component
  unhealthy, and the runner keeps retrying.
- If the producer can stop on its own, set `Exited` so the runner notices. Make
  sure sending to it can't block, for example by using a channel with a buffer
  of one.
- To apply some changes without a restart, set `Reconfigure`. It runs on the
  runner's goroutine, concurrently with the producer's own goroutines.
- To act before the producer stops on shutdown, for example to stop a positions
  file, set `OnShutdown`.

For an example, refer to
[`loki.source.cloudflare`](../../internal/component/loki/source/cloudflare/cloudflare.go).

## Processors

Processors don't need the runner, but the same rules apply:

- Switch downstream components with `loki.Fanout.UpdateChildren`. It never
  blocks.
- Don't wait in `Update` for the processing goroutine to finish or drain, and
  don't hold a lock in `Update` that the processing goroutine holds while it
  sends.
- On shutdown, hand off the entry you hold within a timeout, using one `Send`
  with a context that's canceled after the timeout. Don't retry a `Send` that
  was canceled: it may already have reached some of the receivers.

## Test with the simulation

[`internal/component/common/loki/simulation`](../../internal/component/common/loki/simulation)
runs the real Alloy runtime with fixture components, reloads configs while
entries flow, and checks that:

- no reload deadlocks,
- every entry reaches each sink exactly once per path,
- only removed components drop entries, and
- every source ends up running its latest config, and healthy.
- once that's the case, every entry reaches exactly the sinks, and the paths,
  that the config sends it to.

`TestScenarios` covers the known failure modes. `TestLegacyLifecycles` shows
that the lifecycles most components used before `SourceRunner` fail them.
`TestRandomReloads` runs random sequences of reloads. To soak it, run:

```sh
LOKISIM_SEEDS=200 LOKISIM_STEPS=200 go test -race -run TestRandomReloads ./internal/component/common/loki/simulation/
```

When you add a source archetype the fixtures don't model yet, add it to the
fixtures and the scenarios.

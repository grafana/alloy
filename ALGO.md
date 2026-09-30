# Size-aware target allocation: requirements and options

Working design for milestone 3. This document starts with requirements and open
questions. The immediate next step is pure simulation of the existing TA
consistent-hashing strategy with synthetic targets, measuring balance and movement.
No adjustment layer, peeking, timers or cluster work is needed for this step.
Hashing plus sticky overrides remains a possible later experiment; the detailed
controls below are ideas to revisit after observing the baseline, not an
implementation checklist. We will update the comparison as we implement and
measure candidates. TA remains standalone, with Alloy consuming its assignments
through HTTP discovery.

The initial tests live in the operator checkout at
`cmd/otel-allocator/internal/allocation/simulation_test.go`. They call the existing
production strategy directly; synthetic series counts are measurement inputs only.
Run from `~/workspace/opentelemetry-operator`:

```sh
env -u GOROOT GOMAXPROCS=2 GOMEMLIMIT=1GiB go test -p=2 -mod=readonly \
  ./cmd/otel-allocator/internal/allocation -run '^TestAllocationSimulation' -count=1 -v
```

Assert complete valid assignment, repeatability, order independence and unchanged
owners for surviving targets when only targets are added/removed. Measure moves
on collector scaling/replacement and load imbalance with equal/skewed sizes.
Poor weighted balance is a baseline observation, not a failing test for a strategy
that does not consume weights. These strategy tests do not test discovery, HTTP,
concurrency, staleness or real scrape cost.

Initial baseline (fixed synthetic addresses, three collectors):

| Scenario | Observed result |
| --- | --- |
| 27 equal targets, 10 series each | Collector loads 170 / 50 / 50; max/mean 1.889 |
| Three 800-series targets plus 24 ten-series targets | Collector loads 1750 / 50 / 840; max/mean 1.989; ideal 880 each |
| 1,000 targets, collectors 3 → 4 | 336 targets move |
| 1,000 targets, collectors 3 → 2 | 330 targets move |
| 1,000 targets, one collector replaced at N = 3 | 504 targets move |

Repeated unchanged inputs, input reordering and size-only changes cause zero
moves. Adding/removing targets does not move surviving targets. These are results
for these fixtures, not statistical guarantees. All four simulation tests pass;
the package reported about three seconds after dependency compilation.

## What we want to demonstrate

Balance estimated scrape cost across Alloy instances substantially better than
size-blind allocation, without delaying a target's first scrape or repeatedly
moving targets. Keep discovery centralized and make decisions explainable.

The current workload has three 800-series targets and 24 ten-series targets.
For three equal-capacity scrapers, 880 series each is feasible. Targets are
indivisible: an allocator cannot split one target's scrape among peers. For total
estimated load S, K peers and largest target size W, the best possible maximum
load is at least `max(S/K, W)`. Do not keep moving targets to chase an impossible
balance.

## Requirements

### 1. Asynchronous, inexpensive size estimation

- Assign new targets immediately using a nonzero fallback estimate. Schedule a
  background peek; do not wait for it before serving an assignment or scraping.
- Estimate samples/series per scrape, not the number of metric names or metric
  families. Histograms and labels can produce many series for one metric name.
  Series count is a useful first cost proxy, not a direct measure of CPU or RAM.
- The peek should use the actual target address, path, parameters, authentication
  and negotiated exposition format where applicable. Start with our unauthenticated
  text-format demo; document unsupported cases instead of silently guessing.
- Bound concurrent peeks, request rate, timeout, response bytes and decompressed
  bytes. Stream the response rather than retain the full body. Deduplicate work
  per target, add refresh jitter, and cancel obsolete work when targets disappear.
- A timeout, failed request, unsupported format or truncated response is not size
  zero. Keep the last usable estimate or fallback and expose its age/confidence.
  A partial count is a lower bound unless we have a defensible extrapolation.
- Peeks cost an extra request to the exporter. Cheap parsing does not necessarily
  make response generation or network transfer cheap. Measure both TA overhead
  and the extra exporter traffic before claiming savings.
- Refresh estimates asynchronously. Smooth noise and require meaningful change
  before considering reassignment. Exact smoothing and refresh policy are open.
- A late result must not resurrect a deleted target or overwrite a newer result.
  Tie each result to a target identity/incarnation and observation generation.

Possible signals to compare:

| Signal | Advantage | Limitation / question |
| --- | --- | --- |
| Stream and count sample records | Closest cheap proxy to exposed sample count | Must distinguish comments/blank lines and handle the actual format; text line counting does not cover protobuf/native histograms |
| Response bytes | Minimal parsing | Label lengths, compression and format skew the estimate; Content-Length may be absent; HEAD need not match a GET |
| Count selected characters or sample a prefix | Potentially less parsing/transfer | Heuristic only; format and truncation bias must be measured, and the exporter may still generate the full response |
| Parse exposition properly | More reliable sample accounting | More CPU and implementation work; useful as the reference for estimator error |
| Existing scraper observations | Avoids an extra exporter scrape | Needs a feedback path or backend query; asynchronous alternative for later comparison |

For scraper feedback, `scrape_samples_scraped` measures exposed samples;
`scrape_series_added` measures newly added series and is not total target size.
For now, asynchronous peeking is the requested starting direction, not a commitment
to a particular counting trick. Compare estimator accuracy and cost on the same
responses before choosing one.

### 2. Consistent in-memory state and bounded reader blocking

- Every published assignment generation maps each discovered, eligible target to
  exactly one eligible collector when at least one collector exists. With no
  collectors, explicitly mark targets unassigned; do not fabricate an owner.
- Keep estimates, target membership, collector membership and assignment indexes
  consistent. A reader must see a complete old or complete new generation.
- Use an RW-lock publication model as the starting design: readers obtain a stable
  snapshot; writers publish a complete update under an exclusive lock. No network
  requests, peeking or HTTP serialization while holding that lock.
- Prefer computing a candidate plan from a copied input snapshot outside the write
  lock. Before committing, check its input generation; discard/recompute if any
  planning input changed, including membership or accepted size estimates. Use one
  planner and a coalesced dirty signal, not a queue of historical snapshots or plans.
  Each attempt reads the latest state; see section 6 for membership changes.
- Snapshot data must remain immutable after releasing the read lock. Copying a map
  or slice of mutable pointers is not sufficient by itself. Audit the existing TA
  getter/item mutation paths before adding concurrent size updates.
- Include an internal assignment generation and reason in debug output. Serving
  coherent snapshots is required; changing the standard HTTP SD response is not.

TA already has an `RWMutex` protecting collectors, target items and assignment
indexes. `SetTargets` and `SetCollectors` take its write lock;
`GetTargetsForCollectorAndJob` takes its read lock and returns item pointers.
Reuse or adapt this machinery rather than add an unrelated assignment store.

Atomic publication is **not** atomic handoff across Alloy instances. Independent
HTTP polling means old and new assignments can coexist temporarily. The demo must
measure convergence, overlap and gaps separately from in-memory consistency.

### 3. Size-aware placement and explicit rebalance triggers

- Balance total estimated load across jobs on each collector, initially assuming
  equal collector capacity. Keep per-job target identity so separate scrape jobs
  are not accidentally merged.
- Handle target additions/removals, material estimate changes, and collector
  additions/removals. A change in available Alloy instances must trigger planning.
- Reassign targets from a departed collector promptly. A joining collector must
  eventually receive work; infinite stickiness must not leave it idle forever.
- Coalesce bursts of events. Membership changes should not wait indefinitely for
  the ordinary size-rebalance timer. Distinguish actual discovery removal from
  an individual failed peek or transient collector readiness change.
- Sort targets largest first and place on the least-loaded collector as one
  candidate. Define deterministic target and collector tie-breaking; Go map
  iteration order must not change placement.
- Preserve assignments while new estimates arrive unless a move is justified.
  Recomputing a candidate plan does not require publishing every changed owner.
- Preserve target labels and HTTP SD compatibility. Avoid Alloy-specific protocol
  assumptions so the strategy can remain useful to OTel collectors too.

### 4. Stability: avoid flicker and unnecessary moves

- Proposed starting limit: **at most one assignment publication per second**,
  configurable. This is a ceiling, not a requirement to move targets every second.
  Skip planning when nothing relevant changed; do not publish an unchanged plan.
- Apply the limit globally to the assignment generation, not separately per target,
  job or event producer. Allow the first publication immediately; space subsequent
  publications by at least the configured interval. Do not replay missed ticks or
  accumulate a backlog after a slow calculation.
- Coalesce changes during that interval, then plan from a fresh snapshot when the
  planner is allowed to run. Continuous events must not keep postponing the next
  attempt, as a trailing-edge debounce would. A discarded stale plan leaves the
  state dirty for another fresh attempt; it is never published to catch up.
- One-second publication is only a starting experiment. Alloy currently polls HTTP
  discovery every ten seconds, so peers can skip intermediate generations. This
  rate limit cannot substitute for hysteresis, cooldowns or handoff handling.
- With unchanged membership and stable sizes, repeated evaluations must produce
  no moves. Small measurement noise must not cause back-and-forth moves.
- Balance improvement must justify movement cost. Track both moved target count
  and estimated moved load: moving one huge target can rebuild a large cache.
- Candidate controls: smoothing, minimum improvement/hysteresis, per-target
  cooldown, a move budget, and a preference for current owners. Apart from the
  proposed one-second publication limit, their values and interactions are open.
- Failure recovery must override optional-movement limits when an owner is gone,
  but initially uses the same publication-rate limit: handle it at the next allowed
  opportunity. Any emergency exception to that rate limit needs an explicit decision.
  Collector additions may rebalance gradually, with a bounded convergence goal.
- Avoid moving the same target on successive rounds. Record why each move happened
  and why a proposed improvement was skipped.
- Define restart behavior. In-memory assignments and estimates are lost on a TA
  restart unless persisted or reconstructed. Deterministic fallback helps, but
  cannot guarantee retention of an arbitrary learned assignment. Persistence/HA
  can be deferred for the hackathon if restart churn is visible and documented.

### 5. Target disappearance, handoff and staleness

Treat these as different events:

| Event | Expected responsibility / unresolved behavior |
| --- | --- |
| Target removed from discovery | TA removes its assignment; the previous scraper learns this on its next successful poll |
| Target moves between collectors | Old scraper removes it while new scraper adds it; their actions are not synchronized |
| Peek fails | Keep target membership and a conservative estimate; peeking is not discovery authority |
| TA/HTTP discovery fails | Existing Alloy discovery retains its previous targets; an HTTP failure differs from a successful empty list |

TA can detect target disappearance from its discovery input, but it does not know
all previously emitted series labels, timestamps or downstream relabeling. A size
estimate is insufficient to emit per-series stale markers. TA also has no current
remote-write path. Implementing marker emission there would add scraper-like state
and credentials; leave it out of the initial algorithm work.

Staleness is normally handled by the scraper. Do not assume Alloy clustering's
special suppression of stale markers during ownership handoff also applies to our
HTTP-discovered assignments with clustering disabled. Test the actual path.

Our demo adds `scraper` as a remote-write label. A move therefore creates different
series identities for old and new owners. Old-owner samples can remain queryable
and inflate sums temporarily. Without that label, handoff instead raises questions
about competing samples/stale markers for the same series. Keep this distinction
explicit when interpreting results.

Acceptable initial compromise: make moves infrequent, expose their timing, and
compare settled windows. If stale markers are absent, Prometheus instant selectors
normally stop selecting old samples after the configured lookback (five minutes
by default). This is **not storage garbage collection**: historical samples remain,
and range queries can still include them. Verify the actual Cloud/backend setting;
waiting does not repair scrape gaps or guarantee memory/cardinality accounting has
settled. [Prometheus staleness documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/#staleness).

Proposed later experiment: TA includes a moved-away target in the old collector's
HTTP discovery response with a reserved control label, for example
`__alloy_target_moved="true"` (proposed name, not an existing feature). Alloy treats
that entry as a handoff notice: match the previous local scrape target, disable
its end-of-run stale markers, then exclude it from the new scrape set. The new
collector receives the normal target. A genuinely deleted target is removed
normally and still produces stale markers at its previous owner.

The existing Alloy clustering path already calls
`DisableEndOfRunStalenessMarkers` before handing the new target set to the scrape
manager; an explicit handoff label could reuse this mechanism. It requires Alloy
code support, not just adding a label in TA. Consume the control label before
target identity/scrape processing so it does not create a different scrape target.

The notice must reach the old owner, not only the new one. HTTP discovery polls
can skip assignment generations, so sending it for only one generation is not
reliable; notice retention/delivery remains to be decided. This suppresses the
old owner's end-of-run markers, but does not synchronize scrape start/stop or fix
the demo's different `scraper` series labels. Keep this outside the initial pure
algorithm simulations. TA's RW lock does not solve handoff delivery.

### 6. Alloy membership changes and fresh-state reassignment

The number of eligible collectors, N, is a changing input, not a value captured
when an update was queued. Membership means the collector identities as well as
the count: replacing one collector can require reassignment even if N is unchanged.

- Watchers update the authoritative in-memory view of eligible collectors and mark
  planning dirty. Notifications mean “state changed,” not “execute this old plan.”
  Target discovery and accepted estimate updates follow the same pattern.
- At the next allowed planning opportunity, take a fresh snapshot of collectors,
  targets, estimates and current assignments with an input generation. Derive N
  from that snapshot. Fresh means the latest state known to TA; Kubernetes watch
  delivery and readiness detection still have latency.
- Calculate outside the write lock. Reacquire it and verify the input generation
  before publishing. If membership or another planning input changed meanwhile,
  discard the candidate and plan again from current state. Never commit against an
  obsolete N or leave a new generation assigning work to a departed collector.
- Keep input and published-assignment generations distinct: several input changes
  may collapse into one publication. During that bounded interval, readers may
  still see the previous complete assignment. State atomicity does not promise
  instantaneous failure recovery or simultaneous scraper handoff.
- Changes to N must trigger planning even when no target or size changed. Preserve
  surviving owners where possible, redistribute departed owners' targets, and let
  added collectors absorb work. With N = 0, publish an explicit unassigned state;
  when collectors return, use the latest membership to resume allocation.
- Evaluate eligibility from current readiness/deletion state. A grace period for
  transient unready pods is separate from the one-second publication limiter.
  Track collector incarnations where needed so an old event for a deleted pod
  cannot overwrite the replacement's state under a reused name.
- Capture skipped/coalesced events, stale-plan retries, planning duration and time
  from observed membership change to publication. If input churn repeatedly
  invalidates plans, expose that starvation and choose a bounded retry/fallback
  policy; do not solve it by publishing a stale plan.

Example: TA observes membership change 3 → 4 → 2 within one interval. When the
planner runs, it uses the current two collectors; it does not publish the queued
four-collector arrangement first. If a third collector appears while that plan is
being computed, the generation check rejects it and the next attempt uses all
three current identities. Tests must also cover a replacement where N stays at 3.

### 7. Multiple TA replicas (deferred)

Decision: use Kubernetes Lease leader election if we add HA, likely with three TA
instances and one authoritative allocator. Rapid changes to size estimates and
discovered targets make independent replicas harder to keep in agreement. Keep
one TA instance for the demo. HA implementation is optional later work, not a
requirement for the demo or the current algorithm simulations.

Current TA HA uses independently
computed consistent-hash assignments behind a Service, not leader election or
replicated assignment state. The API explicitly identifies consistent hashing as
[HA-compatible](https://github.com/open-telemetry/opentelemetry-operator/blob/main/apis/v1beta1/targetallocator_types.go).
Equal configuration and collector membership yield equal ownership; replicas can
temporarily disagree while their discovery/watch views differ. Each runs discovery.

Independent async measurements and sticky overrides introduce additional state
that deterministic hashing alone cannot synchronize. With the chosen approach,
only the leader peeks, adjusts and provides authoritative assignments. Routing
remains undecided: a Service could route directly to the leader, or followers
could forward requests. Leader-only readiness affects Deployment rollouts as well
as routing and needs care. The former leader must reject assignment requests on
leadership loss, since routing changes are asynchronous.

Failover initially could rebuild state, accepting reassignment; preserving learned
assignments and outstanding handoff notices requires a shared snapshot or state
replication. Election alone does not preserve them. The new leader must finish
initialization before serving, and the previous leader must stop serving on lease
loss. [client-go leader election](https://pkg.go.dev/k8s.io/client-go/tools/leaderelection)
does not provide strict fencing by itself. These are HA design notes, not work
required for the current pure algorithm simulations.

## Algorithm options to compare

These are design candidates, not claims of a selected or implemented algorithm.

| Option | Balance potential | Stability / implementation trade-off |
| --- | --- | --- |
| Existing consistent hashing | Size-blind reference | Preserve as baseline; membership affects ownership |
| Existing least-weighted | Counts targets, not series | Preserves existing owners; our rolling-switch run left a registered peer idle |
| Full largest-first greedy placement (LPT-style) | Useful size-aware reference: sort descending, assign to least estimated load | Simple full-plan baseline, but small input changes can alter many assignments |
| Sticky weighted greedy with local improvements | Place new/orphaned targets by load; improve overload with selected moves | Natural fit for move budgets/hysteresis; may get stuck and may need swaps |
| Full greedy candidate plus movement penalty/budget | Compare an improved plan with current owners, accepting worthwhile changes | Budgeted partial moves may not achieve the full plan's balance; evaluate actual accepted result |
| Weighted hashing with bounded load | Candidate for retaining hash-based placement preferences | More design work around indivisible weights, overload thresholds and membership changes |

Do not choose a winner from intuition alone. Start by implementing comparable pure
planning functions over the same inputs: targets, estimated weights, collectors,
current owners and deterministic ordering. Keep peeking and HTTP serving outside
the algorithm comparison. A strategy may need changes to TA's allocator lifecycle:
its current interface chooses an owner per target, whereas sorting/planning and
size-update triggers need a view of the full set.

## Prior art: weights, hashing and bounded movement

Distinguish collector capacity weights (a collector should receive twice as much
work) from target cost weights (one target produces far more series). We currently
need the latter, with indivisible targets and asynchronously revised estimates.

| Prior art | What it provides | Fit and limitations for this experiment |
| --- | --- | --- |
| [Consistent Hashing with Bounded Loads](https://arxiv.org/abs/1608.01350), Mirrokni, Thorup and Zadimoghaddam | Hash-based placement with capacity limits and bounds on movement as clients/servers change | Strong starting point for stable placement preferences plus overflow; its unit-sized balls do not directly establish guarantees for our unequal target costs |
| [Weighted rendezvous / HRW, IETF draft](https://www.ietf.org/archive/id/draft-ietf-bess-weighted-hrw-02.html) | Stable mapping with unequal server weights | Useful for unequal collector capacities or deterministic candidate ranking; weighting servers alone does not balance unequal target costs. This reference is a draft, not a final standard |
| [Online Scheduling with Bounded Migration](https://page.math.tu-berlin.de/~skutella/ICALP04-final.pdf), Sanders, Sivadasan and Skutella | Assigns variable-sized jobs while bounding the total size of migrated jobs | Closely matches load versus movement cost. Its job-arrival model with fixed identical machines needs adaptation for collector churn and revised size estimates |

The checked-out TA already uses
[buraksezer/consistent](https://github.com/buraksezer/consistent), a bounded-load
hashing library. Its
[strategy configuration](https://github.com/open-telemetry/opentelemetry-operator/blob/e9e4c53cb656f932d42a31a56d7860bf0987b7e9/cmd/otel-allocator/internal/allocation/consistent_hashing.go)
sets 1,061 partitions, replication factor 5 and load factor 1.1. The library balances
partition ownership; targets map to those partitions by hash. That bound does not
bound target count or series load. Changing the load factor alone cannot introduce
target-cost awareness.

A candidate synthesis for us, not an algorithm selected from these papers:

1. Use stable hashing for a target's preferred owner or ordered candidates, with
   fallback sizes allowing immediate placement.
2. Retain current owners while their total estimated load is acceptable.
3. When fresh estimates or membership changes reveal overload, select a limited
   set of moves to underloaded collectors. Use deterministic choices, hysteresis
   and budgets for both target count and moved estimated load.
4. Keep those exceptions sticky; do not automatically move targets back to their
   hashed home on the next round. Evaluate the benefit before any return move.

This preserves hash-based preferences while adding stateful load correction; it
does not inherit a proof of minimal movement or the unit-ball load bound. A strict
cap near mean load may be impossible with indivisible targets: three targets of
weight 6 on two collectors require a maximum load of 12 although the mean is 9.
Define best-effort placement or a relaxed threshold when no collector fits; never
drop a target to satisfy the cap. Virtual copies of a large target do not solve
this unless the actual scrape work can also be split.

## Leading candidate: existing hashing plus automatic adjustments

Reuse the existing consistent-hashing strategy as the base placement function.
Add a stateful adjustment layer in TA, driven by asynchronous size estimates:

```text
base_owner(target) = existing consistent hash over current collectors
effective_owner(target) = valid override[target], otherwise base_owner(target)
collector_load = sum of estimated target costs using effective owners
```

Overrides are keyed by full target identity, not just address. They persist across
planning rounds. The hash ring does not change when a size estimate changes.
HTTP handlers serve a complete published effective assignment, rather than
independently reading a live ring and override map that could disagree.

At a planning opportunity (at most one publication per second), use the fresh
snapshot described in requirement 6. Start from the base mapping plus retained
overrides, calculate effective loads, and consider moves only when imbalance
exceeds an agreed threshold. A move creates or changes an override; if its
destination is already the base owner, the redundant override can be removed.
Simulate each move against the working loads before accepting it. Multiple targets
may move in one publication: there is no proposed one-target-per-second limit.
Stop when balance is acceptable or no worthwhile move is found, with a bounded
planning-work limit to keep computation responsive. Publish the resulting batch
atomically after generation validation. The priority is avoiding back-and-forth
movement, not artificially spreading useful moves across many publications.

The first move-selection candidate is greedy reduction of the sum of squared
collector loads. After each selected move, update the simulated loads before
selecting another. Require a meaningful improvement and deterministic tie-breaking;
never accept equal-score moves. Consider each target for at most one move per
batch. With fixed estimates and membership, strict objective improvement prevents
cycles; changing estimates still require explicit anti-oscillation controls.

Retain sticky overrides, smooth noisy estimates, and use separate imbalance
thresholds for starting corrections and stopping them. A per-target cooldown
after publication can prevent immediate reversal while estimates settle; its
duration and any substantial-change exception remain to be chosen. Owner failure
bypasses optional-movement restrictions. A movement-count or moved-load cap can
remain an optional operational safeguard, but is not the primary stability control.

The key stability rule: do not clear overrides and rebalance from scratch every
round. Returning to the hash owner is a real move and needs the same justification
as any other optional move. Removing a redundant override without changing the
effective owner is only bookkeeping.

Membership behavior for the initial candidate:

- Rebuild the base mapping from current eligible collectors. Non-overridden targets
  follow the existing hashing strategy's membership behavior.
- Retain overrides whose collectors still exist. Remove overrides for disappeared
  targets or collectors; targets with departed override owners fall back to their
  new base owners. With no collectors, targets remain unassigned.
- Recalculate loads and apply corrections to the resulting effective mapping.
  New collectors can receive hashed targets immediately and overridden work through
  later corrections. Count all effective ownership changes, including base-map
  changes, when reporting churn. If an optional movement budget is configured,
  base-map moves count against it before corrections; departed-owner recovery
  must not be blocked by that budget.

This gives a reusable base strategy and an independently testable correction
policy. Disabling corrections must have explicit semantics: removing existing
overrides returns to base placement and can itself cause a burst of moves.
Restart loses in-memory overrides unless we add persistence; deterministic hash
placement provides a fallback, not preservation of the learned arrangement.

Still to choose: improvement and imbalance thresholds, smoothing, cooldowns and
planning-work limits. Compare batches of greedy single-target moves first;
swaps may be needed when no individual move improves an otherwise poor arrangement.
Do not assume moving the largest target always improves balance.

## Evaluation and acceptance criteria to refine

- Correctness: assignment union equals eligible targets, no duplicate ownership in
  one published generation, and no assignments to missing collectors.
- Balance: max/mean estimated load and observed scrape samples, including idle
  peers; compare with the indivisible-target lower bound.
- Stability: moves per event and interval, moved estimated load, repeated moves of
  the same target, and convergence time after scaling or a real size change.
- Estimation: error versus parsed/sample-count reference, failed/truncated peeks,
  estimate age, peek bytes/second, exporter request rate, TA CPU and memory.
- Service impact: first-scrape delay, HTTP SD latency during updates, actual
  overlap/gaps, and fresh Cloud ownership after convergence.
- Scenarios: unknown targets, heavy skew, ties, noisy or spiking sizes, failed
  peeks, target disappearance/reappearance, scale up/down, zero collectors, and
  TA restart. Include rapid 3 → 4 → 2 membership changes, same-count replacements,
  and a membership change during planning. Assert fresh-state publication, no
  queued-plan replay, and at least one second between publications under an event
  burst. Include mixed formats in estimator tests before claiming support.
- Start with small deterministic simulations and focused race tests. Reuse the
  current single-node demo for live validation; no large local test suites.

## Open decisions for the next discussion

1. Which size signal gives enough accuracy for how much extra exporter work?
   Full streamed text count first, or a bounded approximation from the start?
2. What fallback should an unknown target receive: fixed cost or a job median?
   How long is an old successful estimate trusted after failures?
3. What imbalance and improvement thresholds, smoothing and cooldown prevent
   back-and-forth moves while allowing useful batch corrections? Movement budgets
   are optional safeguards, not a requirement to move only one target at a time.
4. For the hash-plus-overrides candidate, which move-selection objective should
   we try first, and when should swaps be considered? Keep full largest-first
   placement as a comparison for achievable balance, not the default update path.
5. How quickly must scale-up absorb load, and what collector failure grace period
   avoids flicker without delaying recovery too much? Start with the shared
   one-second publication limit; decide whether measured recovery needs an exception.
6. Is the initial staleness compromise above sufficient for the hackathon? Keep
   stale-marker emission out of TA unless we explicitly expand its role.

## Source notes

Reviewed the local operator checkout at
`e9e4c53cb656f932d42a31a56d7860bf0987b7e9`:
[allocator locking and update paths](https://github.com/open-telemetry/opentelemetry-operator/blob/e9e4c53cb656f932d42a31a56d7860bf0987b7e9/cmd/otel-allocator/internal/allocation/allocator.go),
[least-weighted stickiness](https://github.com/open-telemetry/opentelemetry-operator/blob/e9e4c53cb656f932d42a31a56d7860bf0987b7e9/cmd/otel-allocator/internal/allocation/least_weighted.go).
Alloy's [scrape documentation](docs/sources/reference/components/prometheus/prometheus.scrape.md)
describes scrape sample metrics and clustering-specific stale-marker behavior.

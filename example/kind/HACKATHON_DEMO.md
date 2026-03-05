# Preemptive Telemetry -- Demo Script (~3 min)

**Theme:** Minority Report -- "We can see the incident before it happens."

**Format:** Conversation between an SRE and the Grafana Assistant (chat bubbles or split screen).

## The Problem (40s)

*Setting: an SRE gets paged, opens Grafana, starts chatting with the Assistant.*

**SRE:** "Hey, ShopDBMemoryHigh just fired. What's going on?"

**Assistant:** "Memory on shop-db-25jn6 spiked to 95% of its limit before the pod OOM-killed and restarted. The alert has resolved."

**SRE:** "OK, but why? Can you check the cache metrics?"

**Assistant:** "I can't find any `query_cache_size_bytes` or `query_cache_entries_total` data for shop-db. It looks like those metrics were dropped from the pipeline."

**SRE:** "What? Since when?"

**Assistant:** "Since you asked me to optimize costs three weeks ago. I removed debug-level metrics to cut ingestion by 40%. You approved the PR."

*SRE opens the dashboard, expands "Debug Metrics" row -- empty panels.*

**SRE:** "...fine. What about profiles? Can we at least see what was allocating?"

**Assistant:** "Profiling is not configured for shop-db. It was disabled in the same cost optimization."

**SRE:** "Debug logs?"

**Assistant:** "All DEBUG-level log lines are dropped by the Alloy pipeline. I see zero results for shop-db debug logs in the last hour."

**SRE:** "So we have nothing."

**Assistant:** "Correct. The pod restarted, so the in-process state is gone too. I'd suggest we wait for it to happen again."

**SRE:** *stares*

## The Reveal (15s)

*Minority Report clip: precogs seeing the future / dramatic reveal*

**Voiceover:** "What if we could see the incident coming... and automatically gather the evidence before it fully develops?"

Title card: **Preemptive Telemetry**

## The Product (30s)

Show the PT Controller UI at localhost:8090.

**Voiceover:** "Preemptive Telemetry changes that. You configure a simple rule: when this alert goes pending, switch these namespaces to debug-level telemetry."

Walk through the UI:

1. Select alert: ShopDBMemoryHigh
2. Checkboxes auto-select: shop-db
3. Condition: pending
4. TTL: 15m
5. Click Save

**Voiceover:** "That's it. The system watches the alert and acts before you even get paged."

## How It Works (30s)

Show the Alloy config with normal_pipeline and debug_pipeline.

**Voiceover:** "Under the hood, Alloy runs two pipelines. Normal: 30s scrape, no debug logs, no profiles. Debug: 15s scrape, all logs, full pprof profiles, plus detailed metrics like cache size that are normally dropped."

Show the `// <-- PT` markers.

**Voiceover:** "When the rule triggers, the controller pushes an updated pipeline to Fleet Management. Within seconds, every Alloy agent picks it up. No restart, no redeploy."

## The Payoff (45s)

*Minority Report clip: precogs showing the vision / "I can see"*

Show the alert going pending. Show the PT controller logs: "alert matches condition, activating."

**Voiceover:** "Memory is rising. The alert goes pending. Preemptive Telemetry activates automatically."

Switch to Grafana. Show:

1. **Debug metrics appearing:** `query_cache_size_bytes` growing unbounded -- this wasn't visible before
2. **Profiles:** heap profile showing the leaking allocation
3. **Debug logs:** cache entries growing, no eviction

**SRE:** "Now there's data. The cache is growing unbounded -- eviction is disabled."

**Assistant:** "Confirmed. The heap profile shows allocations in `queryCache.add()` are never freed. The debug logs show `query_cache_entries` increasing with no eviction. This matches the OOM pattern."

**SRE:** "Got it. That's the root cause."

## Revert (10s)

**Voiceover:** "After the TTL expires, telemetry reverts to normal automatically. Costs go back down. Evidence stays in Grafana."

## Future Work (20s)

**Voiceover:** "Where this goes next:"

- **Assistant-assisted rules:** the Assistant looks at the alert and suggests which telemetry to enable, for how long, and automatically keeps a healthy pod/cluster as a baseline for comparison
- **Telemetry levels as a user concept:** expose "normal", "debug", "minimal" as first-class levels that users can define per workload -- what each level collects, what it costs, and switch between them with one click. These same level definitions become the building block for adaptive telemetry at the edge: Alloy does the filtering locally, but the complexity stays manageable -- each workload is just mapped to a pre-defined, configurable level
- **Alert on symptoms, diagnose with causes:** keep alerting simple and cheap on high-level symptoms, but automatically enable the deep telemetry that finds root causes during the incident window

## Closing (10s)

*Minority Report clip: "The precogs are never wrong"*

**Voiceover:** "Preemptive Telemetry. The incident hasn't happened yet -- but the evidence is already there."

Title card: **Preemptive Telemetry -- powered by Alloy + Fleet Management**

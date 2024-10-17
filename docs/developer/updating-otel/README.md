# Updating OpenTelemetry Collector dependencies

Alloy depends on various OpenTelemetry (Otel) modules such as these:
```
github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter
github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension
go.opentelemetry.io/collector
go.opentelemetry.io/collector/component
go.opentelemetry.io/otel
go.opentelemetry.io/otel/metric
go.opentelemetry.io/otel/sdk
```

The dependencies mostly come from these repositories:

* [opentelemetry-collector](https://github.com/open-telemetry/opentelemetry-collector)
* [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib)
* [opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go)

Unfortunately, updating Otel dependencies is not straightforward:

* Some of the modules in `opentelemetry-collector` come from a [grafana/opentelemetry-collector](https://github.com/grafana/opentelemetry-collector) fork.
  * This is mostly so that we can include metrics of Collector components with the metrics shown under Alloy's `/metrics` endpoint.
* All Collector and Collector-Contrib dependencies should be updated at the same time, because they
  are kept in sync on the same version.
  * E.g. if we use `v0.85.0` of `go.opentelemetry.io/collector`, we also use `v0.85.0` of `spanmetricsconnector`.
  * This is in line with how the Collector itself imports dependencies.
  * It helps us avoid bugs.
  * It makes it easier to communicate to customers the version of Collector which we use in Alloy.
  * Unfortunately, updating everything at once makes it tedious to check if any of our docs or code need updating due to changes in Collector components. A lot of these checks are manual - for example, cross checking the Otel config and Otel documentation between versions.
  * There are some exceptions for modules which don't follow the same versioning. For example, `collector/pdata` is usually on a different version, like `v1.0.0-rcv0013`.

## Updating walkthrough

### Update the Grafana fork of Otel Collector

1. Create a new release branch from the [opentelemetry release branch](https://github.com/open-telemetry/opentelemetry-collector) with a `-grafana` suffix under [grafana/opentelemetry-collector](https://github.com/grafana/opentelemetry-collector). For example, if porting branch `v0.86.0`, make a branch under the fork repo called `0.86-grafana`.
2. Check which branch of the fork repo Alloy currently uses.
3. See what commits were pushed onto that branch to customize it.
4. Create a PR to cherry-pick the same commits to the new branch. See the [changes to the 0.85 branch](https://github.com/grafana/opentelemetry-collector/pull/8) for an example PR.
5. Run `make` on the branch to make sure it builds and that the tests pass.

### Update Alloy's dependencies

1. Make sure we use the same version of Collector and Collector-Contrib for all relevant modules. For example, if we use version `v0.86.0` of Collector, we should also use version `v0.86.0` for all Contrib modules.
2. Update the `replace` directives in the go.mod file to point to the latest commit of the forked release branch. Use a command like this:
   ```
   go mod edit -replace=go.opentelemetry.io/collector=github.com/grafana/opentelemetry-collector@asdf123jkl
   ```
   Repeat this for any other modules where a replacement is necessary. For debugging purposes, you can first have the replace directive pointing to your local repo.
3. Note that sometimes Collector depends on packages with "rc" versions such as `v1.0.0-rcv0013`. This is ok, as long as the go.mod of Collector also references the same versions - for example, [pdata](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.81.0/go.mod#L25) and [featuregate](https://github.com/open-telemetry/opentelemetry-collector/blob/v0.81.0/go.mod#L24).

### Update otelcol Alloy components

1. Note which Otel components are in use by Alloy.
   * For every "otelcol" Alloy component there is usually a corresponding Collector component.
   * For example, the Otel component used by [otelcol.auth.sigv4](https://grafana.com/docs/alloy/latest/reference/components/otelcol.auth.sigv4/) is [sigv4auth](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/sigv4authextension).
   * In some cases we don't use the corresponding Collector component:
     * For example, [otelcol.receiver.prometheus](https://grafana.com/docs/alloy/latest/reference/components/otelcol.receiver.prometheus/) and [otelcol.exporter.prometheus](https://grafana.com/docs/alloy/latest/reference/components/otelcol.exporter.prometheus/).
     * Those components usually have a note like this:
       > NOTE: otelcol.exporter.prometheus is a custom component unrelated to the prometheus exporter from OpenTelemetry Collector.
2. Make a list of the components which have changed since the previously used version.
   1. Go through the changelogs of both [Collector](https://github.com/open-telemetry/opentelemetry-collector/releases) and [Collector-Contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib/releases).
   2. If a component which is in use by Alloy has changed, note it down.
3. For each Otel component which has changed, compare how they changed.
   1. Compare the old and new version of Otel's documentation.
   2. Compare the config.go file to see if new parameters were added.
4. Update Alloy's code and documentation where needed.
   * Pay attention to stability labels:
      * Never lower the stability label in Alloy. E.g. if the stability
       of an Otel component is "alpha", there are cases where it might be
       stable in Alloy and that is ok. Stability labels in Alloy can
       be increased, but not decreased.
      * If the stability level of an Otel component has increased, consult
      the rest of the team on whether the stability of the corresponding
      Alloy component should also be increased.
   * Search the Alloy repository for the old version (e.g. "0.87") to find code and
     documentation which also needs updating.
   * Update the `OTEL_VERSION` parameter in the `docs/sources/_index.md.t` file.
     Then run `make generate-versioned-files`, which will update `docs/sources/_index.md`.
5. Some alloy components reuse OpenTelemetry code, but do not import it:
   * `otelcol.extension.jaeger_remote_sampling`: a lot of this code has
     been copy-pasted from Otel and modified slightly to fit Alloy's needs.
     This component needs to be updated by copy-pasting the new Otel code
     and modifying it again.
6. Note that we don't port every single config option which OpenTelemetry Collector exposes.
   For example, Collector's [oauth2client extension](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/v0.85.0/extension/oauth2clientauthextension) supports `client_id_file` and `client_secret_file`
   parameters. However, Alloy's [otelcol.auth.oauth2](https://grafana.com/docs/alloy/latest/reference/components/otelcol.auth.oauth2/) does not support them because the idiomatic way of doing the same
   in Alloy is to use the local.file component.
7. When updating semantic conventions, check those the changelogs of those repositories for breaking changes:
   * [opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go/releases)
   * [semantic-conventions](https://github.com/open-telemetry/semantic-conventions/releases)
   * [opentelemetry-specification](https://github.com/open-telemetry/opentelemetry-specification/releases)

You can refer to [PR grafana/agent#5290](https://github.com/grafana/agent/pull/5290)
for an example on how to update Alloy.

## Testing

### Testing a tracing pipeline locally

Firstly, start a K6 trace generator to simulate an application instrumented for tracing:
```
cd docs/developer/updating-otel/k6-trace-gen/
docker compose up -d
```

K6 will be configured to send traces on `ENDPOINT=host.docker.internal:4320`.
This means that the local Alloy instance must be configured to accept traces on `0.0.0.0:4320`.

The ["otelcol" components][otelcol-components]  are the only components which use OTel. 
Try to test as many of them as possible using a config file like this one:

[otelcol-components](https://grafana.com/docs/alloy/latest/reference/components/otelcol/)

<details>
  <summary>Example Alloy config</summary>

```grafana-alloy
otelcol.receiver.otlp "default" {
    grpc {
        endpoint = "0.0.0.0:4320"
    }

    output {
        traces  = [otelcol.processor.batch.default.input]
    }
}

otelcol.processor.batch "default" {
    timeout = "5s"
    send_batch_size = 100

    output {
        traces  = [otelcol.processor.tail_sampling.default.input]
    }
}

otelcol.processor.tail_sampling "default" {
  decision_wait               = "5s"
  num_traces                  = 50000
  expected_new_traces_per_sec = 0

  policy {
    name = "test-policy-1"
    type = "probabilistic"

    probabilistic {
      sampling_percentage = 10
    }
  }

  policy {
    name = "test-policy-2"
    type = "status_code"

    status_code {
      status_codes = ["ERROR"]
    }
  }

  output {
    traces = [otelcol.exporter.otlp.default.input]
  }
}

otelcol.exporter.otlp "default" {
    client {
        endpoint = "localhost:4317"
        tls {
            insecure = true
        }
    }
}
```

</details>

Run this file for two types of Alloy instances - an upgraded one, and another one built using the codebase of the `main` branch. Check the following:

* Open `localhost:12345/metrics` in your browser for both Alloy instances.
  * Are new metrics added? Mention them in the changelog.
  * Are metrics missing? Did any metrics change names? If it's intended, mention them in the changelog and the upgrade guide.
* Check the logs for errors or anything else that's suspicious.
* Check Tempo to make sure the traces were received.

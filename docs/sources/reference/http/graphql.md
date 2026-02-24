---
canonical: https://grafana.com/docs/alloy/latest/reference/http/graphql/
description: Learn about the Grafana Alloy GraphQL API
labels:
  stage: experimental
  products:
    - oss
title: GraphQL API
weight: 100
---

# GraphQL API

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

{{< param "FULL_PRODUCT_NAME" >}} exposes a GraphQL API for querying information about a running instance and its components.
You can use the GraphQL API to retrieve build metadata, readiness status, and component health.

Before you begin, ensure you have the following:

- A running {{< param "PRODUCT_NAME" >}} instance.
- The `--feature.graphql.enabled` flag set to `true` when starting {{< param "PRODUCT_NAME" >}}.

## Enable the GraphQL API

The GraphQL API is disabled by default. To enable it, start {{< param "PRODUCT_NAME" >}} with the `--feature.graphql.enabled` flag:

```sh
alloy run --feature.graphql.enabled config.alloy
```

After you enable the API, the `/graphql` endpoint becomes available on the {{< param "PRODUCT_NAME" >}} HTTP server.
By default, this is `http://localhost:12345/graphql`.

## GraphQL playground

{{< param "PRODUCT_NAME" >}} includes an optional interactive GraphQL playground that you can use to explore the schema and run queries.
To enable the playground, use the `--feature.graphql-playground.enabled` flag:

```sh
alloy run --feature.graphql.enabled --feature.graphql-playground.enabled config.alloy
```

After you enable the playground, it's available at `/graphql/playground` on the {{< param "PRODUCT_NAME" >}} HTTP server.
By default, this is `http://localhost:12345/graphql/playground`.

## Schema

The GraphQL API exposes the following queries.

### `alloy`

Information about the running Alloy instance.

#### Arguments

None.

#### Fields

- **`branch`:** The git branch from which this build was created.
- **`buildDate`:** The timestamp of when this build was created.
- **`buildUser`:** The user account that initiated this build.
- **`isReady`:** Whether the Alloy instance is up and running.
- **`revision`:** The git commit hash from which this build was created.
- **`version`:** The semantic version of this Alloy build.

### `components`

All components running in Alloy.

#### Arguments

None.

#### Fields

- **`health`:** Health status of the component.
  - **`message`:** Message of the health status.
  - **`lastUpdated`:** Last updated time of the health status.
- **`id`:** Fully-qualified ID of the component.
- **`name`:** Name of the component.

### `component`

Component by ID.

#### Arguments

- _`id`_ (`ID!`): The id of the component to retrieve.

#### Fields

- **`health`:** Health status of the component.
  - **`message`:** Message of the health status.
  - **`lastUpdated`:** Last updated time of the health status.
- **`id`:** Fully-qualified ID of the component.
- **`name`:** Name of the component.

The query returns `null` if no component matches the given arguments.

## Example queries

To query the API using `curl`, send a `POST` request to the `/graphql` endpoint with a JSON body containing the `query` field:

```sh
curl -X POST http://localhost:12345/graphql \
  -H "Content-Type: application/json" \
  -d '{"query": "{ alloy { version isReady } }"}'
```

The response is a JSON object:

```json
{
  "data": {
    "alloy": {
      "version": "v1.14.0",
      "isReady": true
    }
  }
}
```

## API details

The GraphQL API has the following behavior:

- **Supported transports:** `OPTIONS`, `GET`, and `POST`.
- **Introspection:** Enabled. You can query the schema using standard GraphQL introspection queries.
- **Timeout:** Each operation has a 10-second timeout.
- **Query caching:** Parsed queries are cached for performance.

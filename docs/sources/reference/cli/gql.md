---
canonical: https://grafana.com/docs/alloy/latest/reference/cli/gql/
description: Learn about the gql command
labels:
  stage: experimental
  products:
    - oss
title: gql
weight: 250
---

# `gql`

{{< docs/shared lookup="stability/experimental_feature.md" source="alloy" version="<ALLOY_VERSION>" >}}

The `gql` command runs a GraphQL query against the [{{< param "PRODUCT_NAME" >}} GraphQL API][GraphQL API].
Use this command to query information from a running {{< param "PRODUCT_NAME" >}} instance, such as build metadata, readiness, and component health.

The `gql` command doesn't enable the GraphQL API.
The target {{< param "PRODUCT_NAME" >}} instance must run with the GraphQL API enabled.

## Usage

```shell
alloy gql [<FLAG> ...] <QUERY>
```

Replace the following:

* _`<FLAG>`_: One or more flags that define the GraphQL endpoint.
* _`<QUERY>`_: Required. The GraphQL query to run.

The `graphql` command is an alias for `gql`.

## Enable the GraphQL API

The GraphQL API is disabled by default.
To enable it, start {{< param "PRODUCT_NAME" >}} with the `--stability.level=experimental` and `--server.http.enable-graphql` flags:

```shell
alloy run --stability.level=experimental --server.http.enable-graphql <CONFIG_PATH>
```

Replace _`<CONFIG_PATH>`_ with the {{< param "PRODUCT_NAME" >}} configuration file or directory path.

## Query formats

The `gql` command accepts complete GraphQL queries.
For example, you can run an anonymous query:

```shell
alloy gql '{ alloy { version isReady } }'
```

The command also accepts query body shorthand.
When the query doesn't start with `{`, `query`, `mutation`, or `subscription`, the command wraps it in `query { ... }`.

For example, the following command:

```shell
alloy gql 'alloy { version isReady }'
```

Runs this GraphQL query:

```graphql
query { alloy { version isReady } }
```

## Flags

The following flags are supported:

* `--endpoint`: Address of the GraphQL endpoint (default `"http://127.0.0.1:12345/graphql"`).

## Examples

To query build and readiness information from the default GraphQL endpoint, use the following command:

```shell
alloy gql 'alloy { isReady version }'
```

The output resembles the following:

```json
{
  "data": {
    "alloy": {
      "isReady": true,
      "version": "v1.14.0"
    }
  }
}
```

To query a GraphQL endpoint on a different address (including other {{< param "PRODUCT_NAME" >}} deployments), use the `--endpoint` flag:

```shell
alloy gql --endpoint=http://localhost:12345/graphql 'components { id health { message } }'
```

[GraphQL API]: ../../http/graphql/

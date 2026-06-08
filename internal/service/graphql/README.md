# Alloy GraphQL Gateway

## How to run

In order to run and/or compile the graphql code, you must generate the graphql stubs based off
the schema file. To do this, you can run either of the following commands:

```
make generate
```
or
```
make generate-graphql
```

From here, you can start Alloy like normal and the GraphQL API will be reachable at
http://localhost:12345/graphql

To enable the GraphQL Playground, run Alloy with `--feature.graphql-playground.enabled` and connect
to the UI at http://localhost:12345/graphql/playground

# Alloy GraphQL Gateway

## How to run

In order to run and/or compile the graphql code, you must generate the graphql stubs based off
the schema file. To do this, you can run either of the following commands:

```
make generate
```
or
```
make graphql
```

From here, you can start Alloy like normal and the GraphQL API will be reachable at
http://localhost:12345/graphql

To enable the GraphQL Playground, export `ALLOY_ENABLE_GRAPHQL_PLAYGROUND=1` and connect to the UI
at http://localhost:12345/graphql/playground

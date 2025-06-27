# Alloy GraphQL Gateway

## How to run

In order to run and/or compile the graphql code, you must generate the graphql stubs based off
the schema file. To do this, you can run either of the following commands:

```
make generate
```
or
```
make generate-graphql-stubs
```

From here, you can run the built-in playground via:

```
cd modules/graphql
go run server.go
```

This runs the graphql playground on port 8080.

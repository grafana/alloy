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

From here, you can start Alloy like normal and connect to the playground at
http://localhost/graphql/playground

The GraphQL API itself is located at http://localhost/graphql

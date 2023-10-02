# graphqltestgen

This is a tool to help generate tests for the Hive [GraphQL simulator](https://github.com/ethereum/hive/tree/master/simulators/ethereum/graphql). The main function is to extend a chain of blocks by writing Go code for the new blocks. For this the genesis and a RLP-encoded list of files should be provided which can be found in the GraphQL test suite. E.g. if you run:

```terminal
go run . --genesis genesis.json --chain chain.rlp
```

it will produce the new chain in `newchain.rlp`. Note that `genesis.json` and `chain.rlp` are the default values so those flags can be skipped.

It's possible to view the latest block of the chain with:

```terminal
go run . --genesis genesis.json --chain chain.rlp head
```

## Filling tests

When adding a new test case it's useful to run the query against a real node to fetch the response. This can be done with the `fill` command as follows:

```terminal
go run . --genesis genesis.json --chain chain.rlp fill --request request.gql --bin /path/to/geth
```

The response will be written to `response.gql`.
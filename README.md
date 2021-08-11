# StreamingFast Golang Client Library

A GraphQL client library to consume dfuse API <https://streamingfast.io> ([dfuse docs](https://docs.dfuse.io)).

## Installation

    go get github.com/streamingfast/client-go

## Features

What you get by using this library:

- StreamingFast API Token issuance & management (auto-refresh, expiration handling, storage, etc)
- StreamingFast GraphQL over gRPC API (planned)
- StreamingFast gRPC API helpers (planned)

## Quick Start

_Notice_ You should replace the sequence of characters `Paste your API key here`
in the script above with your actual API key obtained from https://app.dfuse.io. You are
connecting to a local unauthenticated dfuse instance or to a dfuse Community Edition (EOSIO only)? Replace
`apiKey: "<Paste your API key here>"` pass option `dfuse.WithoutAuthentication` when creating your client.

### Common

```golang
package main

import (

)

func main() {
    client, err := StreamingFast.NewClient("mainnet.eos.dfuse.io", "<Paste your API key here>")
    if err != nil { panic(err) }

    tokenInfo, err := client.GetAPITokenInfo(context.Background())
    if err != nil { panic(err) }

    // Use `tokenInfo.Token` and use it to connect to dfuse's API (`fmt.Sprintf("Bearer %s", tokenInfo.Token)`)
}
```

## References

- [streamingfast Docs](https://docs.dfuse.io)


## Contributing

**Issues and PR in this repo related strictly to the Golang client library.**

Report any protocol-specific issues in their
[respective repositories](https://github.com/streamingfast/streamingfast#protocols)

**Please first refer to the general
[StreamingFast contribution guide](https://github.com/streamingfast/streamingfast/blob/master/CONTRIBUTING.md)**,
if you wish to contribute to this code base.

This codebase uses unit tests extensively, please write and run tests.

## License

[Apache 2.0](LICENSE)

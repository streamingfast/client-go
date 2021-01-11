# dfuse Golang Client Library

A GraphQL client library to consume dfuse API <https://dfuse.io> ([dfuse docs](https://docs.dfuse.io)).

## Installation

    go get github.com/dfuse-io/client-go

## Features

What you get by using this library:

- dfuse API Token issuance & management (auto-refresh, expiration handling, storage, etc)
- dfuse GraphQL over gRPC API (planned)
- dfuse gRPC API helpers (planned)

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
    client, err := dfuse.NewClient("mainnet.eos.dfuse.io", "<Paste your API key here>")
    if err != nil { panic(err) }

    tokenInfo, err := client.GetAPITokenInfo(context.Background())
    if err != nil { panic(err) }

    // Use `tokenInfo.Token` and use it to connect to dfuse's API (`fmt.Sprintf("Bearer %s", tokenInfo.Token)`)
}
```

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our Code of Conduct & processes for submitting pull requests, and [CONVENTIONS.md](CONVENTIONS.md) for our coding conventions.

## License

[Apache 2.0](LICENSE)

## References

- [dfuse Docs](https://docs.dfuse.io)
- [dfuse on Telegram](https://t.me/dfuseAPI) - Community & Team Support

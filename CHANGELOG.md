# Changelog

## Unreleased

- Added `dgql` commane line to easily fetch dfuse GraphQL API from your terminal.

- Initial experimental preview of GraphQL over gRPC directly in the client.

- Removed "global" methods, everything must now be called from a client instance.

- Converted all code to use the second major revision of Go protocol buffer, a.k.a APIv2 ().

- Added support for variables in `client.GraphQLQuery`.

- Initial (work in progress) version of GraphQL over gRPC support.

- Added support for unauthenticated endpoints.

- Token management & persistent storage.

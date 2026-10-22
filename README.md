# inoreader-go

A Go client library for the [Inoreader API](https://www.inoreader.com/developers/).

## Development

Tools and tasks are managed with [mise](https://mise.jdx.dev/). Run `mise install` once, then `mise tasks` to list the tasks.

### API description

[`api/openapi.yaml`](api/openapi.yaml) describes the Inoreader API in OpenAPI 3.1. Inoreader publishes no machine-readable description, so it is written by hand from the developer portal. It is the contract the client is written against, and the source of the mock server.

Lint it after editing:

```sh
mise run api:lint
```

### Mock server

E2E tests run against a mock server that [Prism](https://stoplight.io/open-source/prism) serves from the API description:

```sh
mise run mock                # http://127.0.0.1:4010
mise run mock -- --port 8080
```

Point the client at the mock server's address instead of `https://www.inoreader.com`. The mock server:

- returns the examples in the description, with the rate limit headers
- rejects requests without credentials (`401`) and requests that don't match the description (`400`, with the violations in the `sl-violations` header)
- returns any documented response on request, through the `Prefer` header, e.g. `Prefer: code=429` for a reached rate limit

It keeps no state: marking an article as read does not change what the next request returns.

# Contributing

Bug reports, focused feature requests, documentation improvements, and pull requests are welcome.

## Development setup

ascdir requires Go 1.26.6 or later.

```sh
git clone https://github.com/Arata1202/ascdir.git
cd ascdir
make check
```

Before opening a pull request:

```sh
make fmt
make check
```

Tests must not call the production App Store Connect API or require real credentials. Use `httptest` and generated test keys instead.

Keep pull requests focused and explain any user-visible behavior change in the description.

By participating, you agree to follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

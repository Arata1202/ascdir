# Getting started

Install ascdir, connect it to an existing App Store Connect app, and preview the first change without mutating remote state.

## Install

```sh
brew install Arata1202/tap/ascdir
```

You can also download a signed release artifact from [GitHub Releases](https://github.com/Arata1202/ascdir/releases).

## Configure App Store Connect credentials

Create an App Store Connect API key, then expose its values to ascdir:

```sh
export ASC_ISSUER_ID="your-issuer-id"
export ASC_KEY_ID="your-key-id"
export ASC_PRIVATE_KEY_PATH="$PWD/AuthKey.p8"
```

Keep the private key outside version control. ascdir generates short-lived ES256 tokens locally and does not upload the key.

## Initialize an existing app

Run the command from the repository that will own the App Store configuration:

```sh
ascdir init \
  --bundle-id com.example.app \
  --platform IOS \
  --version 1.2.0
```

Review the generated files before committing them.

## Preview changes

```sh
ascdir push --dry-run
```

Dry run validates local files, reads the current App Store Connect state, and prints the planned operations without applying them.

## Apply reviewed changes

```sh
ascdir push
```

Start with [metadata](./metadata.md), then add screenshots, previews, pricing, availability, TestFlight distribution, and release automation as needed.

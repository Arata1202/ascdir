# ascdir website

The official ascdir site is a statically exported Next.js application. It has no server runtime, API, database, CMS, or duplicated documentation source.

## Local development

```sh
corepack enable
pnpm install
pnpm dev
```

The pages under `/docs/` are generated from the repository-level [`docs/`](../docs/) directory. Keep CLI behavior and its documentation in the same pull request.

## Quality checks

```sh
pnpm check
pnpm exec playwright install chromium
pnpm test:e2e
pnpm lighthouse
```

`pnpm check` creates the production static export first. Playwright serves and tests that exact
`out/` artifact rather than the development server.

## Cloudflare Pages

- Root directory: `website`
- Build command: `pnpm build`
- Output directory: `out`
- Environment variable: `NEXT_PUBLIC_SITE_URL` set to the canonical production origin
- Node.js: 24

Cloudflare Pages copies [`public/_headers`](public/_headers) into the deployment to apply security
and cache headers. Preview deployments are emitted with `noindex` automatically from Cloudflare's
`CF_PAGES` and `CF_PAGES_BRANCH` build variables.

Optional production integrations are configured with `NEXT_PUBLIC_GOOGLE_ANALYTICS_ID`,
`NEXT_PUBLIC_GOOGLE_SITE_VERIFICATION`, and `NEXT_PUBLIC_SENTRY_DSN`. Sentry source-map uploads additionally use `SENTRY_ORG`, `SENTRY_PROJECT`, and `SENTRY_AUTH_TOKEN` in the build environment.

Pull requests receive Cloudflare preview deployments. Only the production deployment should use the canonical custom-domain value.

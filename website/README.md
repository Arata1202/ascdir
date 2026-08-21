# ascdir website

The ascdir landing page is a statically exported Next.js application.

```sh
npm ci
npm run dev
```

Validate the production output before opening a pull request:

```sh
npm run typecheck
npm run build
```

The static site is written to `out/`. It intentionally has no server runtime,
API routes, database, or duplicated reference documentation; detailed usage
stays in the repository README and `docs/` directory.

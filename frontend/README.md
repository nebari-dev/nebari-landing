# Frontend (React + TypeScript + Vite)

The Nebari landing-page SPA. Built with React, TypeScript, Vite, and shadcn/ui.
Talks to the Go webapi (in this same repo) over `/api/v1`.

## React Compiler

Not enabled, due to its impact on dev/build performance. To opt in, see the
[React Compiler installation guide](https://react.dev/learn/react-compiler/installation).

## Lint and format

Linting and formatting are unified under [Biome](https://biomejs.dev/). The
configuration lives in [`biome.json`](./biome.json). Scripts:

- `npm run lint` — `biome check .` (lint + format diagnostics, read-only)
- `npm run lint:fix` — apply safe autofixes
- `npm run format` / `npm run format:write` — format-only (read-only / write)
- `npm run ci` — strict CI variant (`biome ci`), used by the workflow

### Coverage gap: type-checked lint rules

Biome is a **syntactic** linter — it parses source files but does not run the
TypeScript type checker. Rules that need full type information are therefore
unavailable here, including:

- `no-floating-promises` — unhandled `Promise<T>` return values
- `no-misused-promises` — async callbacks passed where a non-Promise is expected
  (e.g. `<button onClick={async () => …}>`)
- `await-thenable`, `no-unnecessary-type-assertion`, and the `no-unsafe-*` family

The previous ESLint config (`tseslint.configs.recommended`) did not enable these
either, so nothing was actively lost in the Biome migration. They are noted here
so contributors don't assume the lint pass catches them — rely on code review
and TypeScript's `strict` mode instead. Biome ships heuristic versions of the
promise rules in its `nursery` group; revisit enabling them once they leave
nursery.

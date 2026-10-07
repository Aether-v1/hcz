# HCZ User Web

The customer-facing storefront for browsing products, placing orders, and completing payments.

> This directory is part of the [hcz](https://github.com/Aether-v1/hcz)
> single repository and is no longer released on its own. Production assets are embedded
> into the server binary via `go:embed` and served by the same process, on the same port,
> as `frontend/admin`.

## Tech Stack

Vue 3 · TypeScript · Vite · Tailwind CSS v4 · Pinia · vue-i18n

## Local Development

```bash
pnpm install
pnpm run dev          # http://localhost:5173
```

Requires the backend to be running: `go run ./cmd/server` from the repository root.

The dev server proxies `/api`, `/uploads`, `/sitemap.xml` and `/robots.txt` to
`localhost:8080` with `changeOrigin: false`, preserving the original Host so the backend
can resolve reseller tenant domains.

### Anonymous UI preview (local development only)

Copy `.env.example` to `.env.development.local` and set
`VITE_DEV_PREVIEW_MODE=true`, then restart the Vite dev server. The current local
checkout already has this ignored local setting. With no user session, selected
pages such as `/me`, `/me/orders`, `/me/wallet`, `/me/invitation`, `/notifications`
and `/support` render a read-only visual preview. The badge says `DEV PREVIEW`.
Private API calls still require backend authentication; empty/error states are
expected. Non-GET API mutations are blocked before any request is sent, except
public authentication endpoints. Unlisted private routes still redirect to login.
The flag is ignored in a production build because the code also requires
`import.meta.env.DEV === true`.

## Build

```bash
pnpm run build
```

Assets are served from the site root `/` through the backend's `NoRoute` fallback.
See `internal/web/handler.go`.

You normally don't run these by hand — `make build-fullstack`, the Docker build, and the
GitHub Actions release workflow all build and embed the frontends for you.

## Documentation




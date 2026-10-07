export interface PreviewEnvironment {
  DEV: boolean
  VITE_DEV_PREVIEW_MODE?: string
}

/** Only a development build with an explicit opt-in may render anonymous previews. */
export const isDevPreviewEnabled = (env: PreviewEnvironment): boolean =>
  env.DEV === true && env.VITE_DEV_PREVIEW_MODE === 'true'

const PUBLIC_PREVIEW_ROUTES = new Set([
  'home', 'products', 'category-products', 'product-detail',
])

const PRIVATE_PREVIEW_ROUTES = new Set([
  'personal-center', 'personal-center-orders', 'personal-center-wallet',
  'personal-center-invitation', 'personal-center-profile', 'profile-edit', 'notifications', 'support-home',
  'support-tickets', 'c2c-home', 'personal-center-gift-cards', 'personal-center-api',
])

export const isAnonymousPreviewRoute = (name: unknown): boolean =>
  typeof name === 'string' && (PUBLIC_PREVIEW_ROUTES.has(name) || PRIVATE_PREVIEW_ROUTES.has(name))

export const isPrivatePreviewRoute = (name: unknown): boolean =>
  typeof name === 'string' && PRIVATE_PREVIEW_ROUTES.has(name)

export const shouldBlockPreviewWrite = (
  previewGuest: boolean,
  method: string,
  publicAuthEndpoint: boolean,
): boolean => previewGuest && method.toUpperCase() !== 'GET' && !publicAuthEndpoint

export const shouldRedirectUnauthorized = (
  authenticatedClient: boolean,
  publicAuthEndpoint: boolean,
  previewGuest: boolean,
): boolean => authenticatedClient && !publicAuthEndpoint && !previewGuest

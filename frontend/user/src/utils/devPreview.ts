import { isDevPreviewEnabled } from './devPreviewPolicy'

export const DEV_PREVIEW_MODE = isDevPreviewEnabled(import.meta.env)

export const isGuestDevPreview = (authenticated: boolean): boolean =>
  DEV_PREVIEW_MODE && !authenticated

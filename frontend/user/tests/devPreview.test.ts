import assert from 'node:assert/strict'
import test from 'node:test'
import {
  isAnonymousPreviewRoute,
  isDevPreviewEnabled,
  isPrivatePreviewRoute,
  shouldBlockPreviewWrite,
  shouldRedirectUnauthorized,
} from '../src/utils/devPreviewPolicy.ts'

test('preview needs both a dev build and an explicit true flag', () => {
  assert.equal(isDevPreviewEnabled({ DEV: true, VITE_DEV_PREVIEW_MODE: 'true' }), true)
  assert.equal(isDevPreviewEnabled({ DEV: true, VITE_DEV_PREVIEW_MODE: 'false' }), false)
  assert.equal(isDevPreviewEnabled({ DEV: true }), false)
  assert.equal(isDevPreviewEnabled({ DEV: false, VITE_DEV_PREVIEW_MODE: 'true' }), false)
})

test('anonymous preview only allows audited visual routes', () => {
  for (const name of ['home', 'products', 'category-products', 'product-detail', 'personal-center', 'personal-center-orders', 'personal-center-wallet', 'personal-center-invitation', 'notifications', 'support-home', 'support-tickets', 'c2c-home']) {
    assert.equal(isAnonymousPreviewRoute(name), true, name)
  }
  for (const name of ['wallet-withdrawal', 'c2c-buy', 'c2c-sell', 'personal-center-affiliate', 'support-ticket-new', 'reseller-finance']) {
    assert.equal(isAnonymousPreviewRoute(name), false, name)
  }
  assert.equal(isPrivatePreviewRoute('personal-center-wallet'), true)
  assert.equal(isPrivatePreviewRoute('products'), false)
})

test('guest preview blocks every non-GET API mutation except public authentication', () => {
  for (const method of ['POST', 'PUT', 'PATCH', 'DELETE']) {
    assert.equal(shouldBlockPreviewWrite(true, method, false), true, method)
  }
  assert.equal(shouldBlockPreviewWrite(true, 'GET', false), false)
  assert.equal(shouldBlockPreviewWrite(true, 'POST', true), false)
  assert.equal(shouldBlockPreviewWrite(false, 'POST', false), false)
})

test('private 401 redirects only outside anonymous preview', () => {
  assert.equal(shouldRedirectUnauthorized(true, false, true), false)
  assert.equal(shouldRedirectUnauthorized(true, false, false), true)
  assert.equal(shouldRedirectUnauthorized(true, true, false), false)
})

import test from 'node:test'
import assert from 'node:assert/strict'
import {
  DEFAULT_BRAND_PRIMARY,
  DEFAULT_HOME_ENTRIES,
  normalizeHomeEntries,
  normalizeDiscoveryBlocks,
  pickText,
  withAlpha,
  isHexColor,
  toBool,
} from '../src/utils/siteConfig.ts'

const ROUTE_MAP: Record<string, string> = {
  recharge: '/products',
  c2c: '/c2c',
  wallet: '/me/wallet',
  withdrawal: '/me/wallet/withdrawal',
  invitation: '/me/invitation',
  support: '/support',
  orders: '/me/orders',
  discovery: '/discovery',
}

test('brand default is indigo-600 and hex validation works', () => {
  assert.equal(DEFAULT_BRAND_PRIMARY, '#4F46E5')
  assert.equal(isHexColor('#4F46E5'), true)
  assert.equal(isHexColor('#fff'), true)
  assert.equal(isHexColor('red'), false)
  assert.equal(isHexColor(''), false)
})

test('withAlpha appends alpha to hex', () => {
  assert.equal(withAlpha('#4F46E5', '1A'), '#4F46E51A')
  assert.equal(withAlpha('#fff', '1A'), '#ffffff1A')
  assert.equal(withAlpha('not-a-color', '1A'), '')
})

test('toBool normalizes truthy strings', () => {
  assert.equal(toBool('yes'), true)
  assert.equal(toBool('false'), false)
  assert.equal(toBool(1), true)
  assert.equal(toBool(0), false)
  assert.equal(toBool(undefined, true), true)
})

test('pickText resolves localized text', () => {
  assert.equal(pickText('plain', 'en-US'), 'plain')
  assert.equal(pickText({ 'zh-CN': '中文', 'en-US': 'EN' }, 'en-US'), 'EN')
  assert.equal(pickText({ 'zh-CN': '中文' }, 'en-US'), '中文')
})

test('home_entries empty falls back to default 4 entries', () => {
  const entries = normalizeHomeEntries([], 'zh-CN', ROUTE_MAP)
  assert.equal(entries.length, DEFAULT_HOME_ENTRIES.length)
  assert.equal(entries[0].key, 'recharge')
  assert.equal(entries[0].href, '/products')
  assert.equal(entries[0].external, false)
  assert.equal(entries[0].recommended, true)
})

test('home_entries filters disabled and sorts by sort_order', () => {
  const raw = [
    { id: 1, key: 'wallet', title: 'Wallet', sort_order: 2, enabled: true, action_type: 'internal' },
    { id: 2, key: 'support', title: 'Support', sort_order: 1, enabled: false, action_type: 'internal' },
    { id: 3, key: 'orders', title: 'Orders', sort_order: 0, enabled: true, action_type: 'internal' },
  ]
  const entries = normalizeHomeEntries(raw, 'zh-CN', ROUTE_MAP)
  // disabled (support) 被过滤；剩余按 sort_order 升序
  assert.deepEqual(entries.map((e) => e.key), ['orders', 'wallet'])
  assert.equal(entries[0].href, '/me/orders')
})

test('home_entries external requires http url', () => {
  const raw = [
    { id: 1, key: 'ext', title: 'Ext', action_type: 'external', action_target: 'https://example.com' },
    { id: 2, key: 'bad', title: 'Bad', action_type: 'external', action_target: '/relative' },
  ]
  const entries = normalizeHomeEntries(raw, 'zh-CN', ROUTE_MAP)
  assert.equal(entries.length, 1)
  assert.equal(entries[0].key, 'ext')
  assert.equal(entries[0].external, true)
})

test('home_entries internal resolves route map by key', () => {
  const raw = [{ id: 1, key: 'invitation', title: 'Invite', action_type: 'internal', enabled: true }]
  const entries = normalizeHomeEntries(raw, 'zh-CN', ROUTE_MAP)
  assert.equal(entries[0].href, '/me/invitation')
})

test('discovery blocks filter enabled and sort', () => {
  const raw = [
    { id: 1, type: 'banner', sort_order: 2, enabled: true },
    { id: 2, type: 'card_grid', sort_order: 1, enabled: false },
    { id: 3, type: 'announcement', sort_order: 0, enabled: true },
  ]
  const blocks = normalizeDiscoveryBlocks(raw, 'zh-CN')
  assert.deepEqual(blocks.map((b) => b.type), ['announcement', 'banner'])
})

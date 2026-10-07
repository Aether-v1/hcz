import assert from 'node:assert/strict'
import test from 'node:test'
import { formatUsdt } from '../src/utils/money.ts'

test('My page USDT amount uses half-up rounding and always shows two decimals', () => {
  assert.equal(formatUsdt('12.345', 'USDT'), '12.35 USDT')
  assert.equal(formatUsdt('12.344', 'USDT'), '12.34 USDT')
  assert.equal(formatUsdt('0', 'USDT'), '0.00 USDT')
})

test('Missing or invalid wallet amount never becomes a fabricated zero', () => {
  assert.equal(formatUsdt(null, 'USDT'), '--')
  assert.equal(formatUsdt('', 'USDT'), '--')
  assert.equal(formatUsdt('bad', 'USDT'), '--')
})

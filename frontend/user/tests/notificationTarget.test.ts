import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveNotificationTarget } from '../src/utils/notificationTarget.ts'

test('notification deep links target validated authenticated pages', () => {
  assert.equal(resolveNotificationTarget({ biz_type: 'order', data: { order_no: 'HCZ_123-4' } }), '/orders/HCZ_123-4')
  assert.equal(resolveNotificationTarget({ biz_type: 'support_ticket', biz_id: 12 }), '/support/tickets/12')
  assert.equal(resolveNotificationTarget({ biz_type: 'c2c_trade', biz_id: 12 }), '/c2c/trades/12')
  assert.equal(resolveNotificationTarget({ biz_type: 'c2c_trade', data: { trade_id: '42' } }), '/c2c/trades/42')
  assert.equal(resolveNotificationTarget({ biz_type: 'order', data: { order_no: '../admin' } }), '')
  assert.equal(resolveNotificationTarget({ biz_type: 'c2c_trade', data: { trade_id: '../admin' }, biz_id: -1 }), '')
  assert.equal(resolveNotificationTarget({ biz_type: 'unknown', biz_id: 1 }), '')
})

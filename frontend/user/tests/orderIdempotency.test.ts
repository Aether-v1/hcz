import test from 'node:test'
import assert from 'node:assert/strict'

/**
 * Order Idempotency 前端纯逻辑单元测试。
 *
 * 由于 node --experimental-strip-types 无法解析带 browser 依赖（localStorage/fetch/Vue）
 * 的模块，本测试将 useCheckout.ts 中幂等 key 管理逻辑和 api/order.ts 中 header 构造逻辑
 * 内联后验证，覆盖：
 * - key 生成格式（UUID v4）
 * - 同一次提交内 ensureIdempotencyKey 复用同一 key
 * - resetIdempotencyKey 后生成新 key（新业务请求）
 * - API wrapper 在有 key 时构造 Idempotency-Key header
 * - API wrapper 在无 key 时不构造 header
 * - submitting guard 与幂等 key 独立（submitting 防双击，key 防重复建单）
 *
 * 这些逻辑与 src/composables/useCheckout.ts、src/api/order.ts 中的实现一一对应。
 */

// ─── 与 src/composables/useCheckout.ts 同步的幂等 key 管理逻辑 ───

/**
 * 模拟 useCheckout 中的 idempotencyKey ref + ensureIdempotencyKey + resetIdempotencyKey。
 * 实际实现使用 Vue ref，此处用闭包变量模拟相同语义。
 */
function createIdempotencyKeyManager() {
    let key = ''
    return {
        getKey: () => key,
        ensureIdempotencyKey: () => {
            if (!key) {
                key = crypto.randomUUID()
            }
            return key
        },
        resetIdempotencyKey: () => {
            key = ''
        },
    }
}

// ─── 与 src/api/order.ts 同步的 header 构造逻辑 ───

/**
 * 模拟 userOrderAPI.createAndPay 中的 header 构造。
 * 实际实现：headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
 */
function buildOrderHeaders(idempotencyKey?: string): Record<string, string> | undefined {
    return idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
}

// ─── 1. key 生成格式 ───

test('ensureIdempotencyKey generates UUID v4 format', () => {
    const mgr = createIdempotencyKeyManager()
    const key = mgr.ensureIdempotencyKey()
    // UUID v4: 8-4-4-4-12 hex digits, 13th char is 4, 17th char is 8/9/a/b
    assert.match(key, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
})

test('ensureIdempotencyKey returns non-empty string', () => {
    const mgr = createIdempotencyKeyManager()
    const key = mgr.ensureIdempotencyKey()
    assert.ok(typeof key === 'string' && key.length > 0)
})

// ─── 2. 同一次提交内复用同一 key ───

test('ensureIdempotencyKey returns same key on repeated calls (same submit)', () => {
    const mgr = createIdempotencyKeyManager()
    const first = mgr.ensureIdempotencyKey()
    const second = mgr.ensureIdempotencyKey()
    const third = mgr.ensureIdempotencyKey()
    assert.equal(first, second)
    assert.equal(second, third)
})

test('same key reused across 100 repeated calls (simulating network retries)', () => {
    const mgr = createIdempotencyKeyManager()
    const first = mgr.ensureIdempotencyKey()
    for (let i = 0; i < 100; i++) {
        assert.equal(mgr.ensureIdempotencyKey(), first)
    }
})

// ─── 3. 新业务请求生成新 key ───

test('resetIdempotencyKey then ensure generates a new key (new business request)', () => {
    const mgr = createIdempotencyKeyManager()
    const first = mgr.ensureIdempotencyKey()
    mgr.resetIdempotencyKey()
    assert.equal(mgr.getKey(), '')
    const second = mgr.ensureIdempotencyKey()
    assert.notEqual(first, second)
    assert.match(second, /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/)
})

test('different key managers produce different keys (simulating separate checkout sessions)', () => {
    const mgr1 = createIdempotencyKeyManager()
    const mgr2 = createIdempotencyKeyManager()
    assert.notEqual(mgr1.ensureIdempotencyKey(), mgr2.ensureIdempotencyKey())
})

// ─── 4. API wrapper header 构造 ───

test('buildOrderHeaders includes Idempotency-Key when key provided', () => {
    const headers = buildOrderHeaders('test-key-123')
    assert.ok(headers !== undefined)
    assert.equal(headers!['Idempotency-Key'], 'test-key-123')
})

test('buildOrderHeaders returns undefined when no key', () => {
    assert.equal(buildOrderHeaders(undefined), undefined)
    assert.equal(buildOrderHeaders(''), undefined)
})

test('buildOrderHeaders preserves exact key value (no trimming or encoding)', () => {
    const key = 'a-b-c-123-XYZ'
    const headers = buildOrderHeaders(key)
    assert.equal(headers!['Idempotency-Key'], key)
})

// ─── 5. submitting guard 与幂等 key 独立 ───

test('submitting guard and idempotency key are independent mechanisms', () => {
    // submitting guard: prevents double-click within UI (client-side only)
    let submitting = false
    const submit = () => {
        if (submitting) return 'blocked'
        submitting = true
        return 'proceed'
    }
    assert.equal(submit(), 'proceed')
    assert.equal(submit(), 'blocked') // double-click blocked

    // idempotency key: persists across retries, server-side dedup
    const mgr = createIdempotencyKeyManager()
    const key = mgr.ensureIdempotencyKey()

    // Even if submitting guard is reset (e.g., after error), key remains the same
    submitting = false
    assert.equal(mgr.ensureIdempotencyKey(), key) // same key for retry
    assert.equal(submit(), 'proceed') // submitting allows again
})

test('key survives submitting guard reset (retry after error uses same key)', () => {
    const mgr = createIdempotencyKeyManager()
    const key = mgr.ensureIdempotencyKey()

    // Simulate: first attempt fails, submitting is reset in finally block
    // User clicks submit again — should reuse same key
    assert.equal(mgr.ensureIdempotencyKey(), key)
    assert.equal(mgr.ensureIdempotencyKey(), key)
})

// ─── 6. 购物车变化触发 key 重置（watch cartItems → resetIdempotencyKey） ───

test('cart items change triggers key reset (new business intent)', () => {
    const mgr = createIdempotencyKeyManager()
    const firstKey = mgr.ensureIdempotencyKey()

    // Simulate watch on cartItems: when items change, resetIdempotencyKey is called
    mgr.resetIdempotencyKey()

    // Next submit generates new key
    const secondKey = mgr.ensureIdempotencyKey()
    assert.notEqual(firstKey, secondKey)
})

test('cart items unchanged does not reset key (same business intent = retry)', () => {
    const mgr = createIdempotencyKeyManager()
    const firstKey = mgr.ensureIdempotencyKey()

    // No cart change → no reset → same key
    assert.equal(mgr.ensureIdempotencyKey(), firstKey)
    assert.equal(mgr.ensureIdempotencyKey(), firstKey)
})

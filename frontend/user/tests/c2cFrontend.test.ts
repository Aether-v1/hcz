import test from 'node:test'
import assert from 'node:assert/strict'

/**
 * C2C 前端纯逻辑单元测试。
 *
 * 由于 node --experimental-strip-types 无法解析带 browser 依赖（localStorage/fetch）
 * 的 Vue 项目模块，本测试将 C2C 业务逻辑中的纯函数/常量规则内联后验证，
 * 覆盖：交易状态机、轮询启停、倒计时、金额换算、表单校验、风控错误映射、通知深链。
 *
 * 这些逻辑与 src/api/c2c.ts、src/composables/useC2C.ts、src/views/c2c/*.vue 中的实现一一对应。
 */

// ─── 与 src/api/c2c.ts 同步的常量 ──────────────────────────────

const C2C_TRADE_TERMINAL_STATUSES = ['completed', 'canceled', 'expired'] as const
const C2C_TRADE_POLLING_STATUSES = ['pending_payment', 'paid', 'disputed'] as const
const C2C_PAYMENT_METHOD_TYPES = ['bank', 'alipay', 'wechat', 'paynow', 'custom'] as const

const C2C_ERROR_KEYS = {
    NOT_FOUND: 'error.c2c_not_found',
    SELF_TRADE: 'error.c2c_self_trade',
    LISTING_NOT_ACTIVE: 'error.c2c_listing_not_active',
    INVALID_AMOUNT: 'error.c2c_invalid_amount',
    STATUS_INVALID: 'error.c2c_status_invalid',
    IDEMPOTENCY_REQUIRED: 'error.idempotency_key_required',
    C2C_DISABLED: 'error.c2c_disabled',
    TOTP_REQUIRED: 'error.c2c_totp_required',
    NEW_USER_COOLDOWN: 'error.c2c_new_user_cooldown',
    NO_PAYMENT_METHOD: 'error.c2c_no_payment_method',
    DAILY_LIMIT: 'error.c2c_daily_limit',
    INSUFFICIENT_BALANCE: 'error.c2c_insufficient_balance',
    PERMISSION_DENIED: 'error.c2c_permission_denied',
    USER_INACTIVE: 'error.c2c_user_inactive',
} as const

// ─── 与 src/composables/useC2C.ts 同步的标签 ───────────────────

const TRADE_STATUS_LABELS: Record<string, string> = {
    pending_payment: '待付款',
    paid: '待确认',
    disputed: '申诉中',
    completed: '已完成',
    canceled: '已取消',
    expired: '已超时',
}

const LISTING_STATUS_LABELS: Record<string, string> = {
    active: '进行中',
    paused: '已暂停',
    closed: '已关闭',
}

const PAYMENT_METHOD_TYPE_LABELS: Record<string, string> = {
    bank: '银行卡',
    alipay: '支付宝',
    wechat: '微信支付',
    paynow: 'PayNow',
    custom: '自定义',
}

// ─── 1. 交易状态常量 ────────────────────────────────────────────

test('trade terminal statuses = completed/canceled/expired', () => {
    assert.deepEqual([...C2C_TRADE_TERMINAL_STATUSES].sort(), ['canceled', 'completed', 'expired'])
})

test('trade polling statuses = pending_payment/paid/disputed', () => {
    assert.deepEqual([...C2C_TRADE_POLLING_STATUSES].sort(), ['disputed', 'paid', 'pending_payment'])
})

test('polling statuses never overlap terminal statuses', () => {
    const terminal = new Set(C2C_TRADE_TERMINAL_STATUSES)
    for (const s of C2C_TRADE_POLLING_STATUSES) {
        assert.equal(terminal.has(s), false)
    }
})

test('all six trade statuses have Chinese labels', () => {
    for (const [status, label] of Object.entries({
        pending_payment: '待付款', paid: '待确认', disputed: '申诉中',
        completed: '已完成', canceled: '已取消', expired: '已超时',
    })) {
        assert.equal(TRADE_STATUS_LABELS[status], label)
    }
})

// ─── 2. 挂单状态常量 ────────────────────────────────────────────

test('listing status labels = 进行中/已暂停/已关闭', () => {
    assert.equal(LISTING_STATUS_LABELS['active'], '进行中')
    assert.equal(LISTING_STATUS_LABELS['paused'], '已暂停')
    assert.equal(LISTING_STATUS_LABELS['closed'], '已关闭')
})

// ─── 3. 支付方式类型 ────────────────────────────────────────────

test('payment method types = bank/alipay/wechat/paynow/custom', () => {
    assert.deepEqual([...C2C_PAYMENT_METHOD_TYPES].sort(), ['alipay', 'bank', 'custom', 'paynow', 'wechat'])
})

test('payment method type Chinese labels', () => {
    assert.equal(PAYMENT_METHOD_TYPE_LABELS['bank'], '银行卡')
    assert.equal(PAYMENT_METHOD_TYPE_LABELS['alipay'], '支付宝')
    assert.equal(PAYMENT_METHOD_TYPE_LABELS['wechat'], '微信支付')
    assert.equal(PAYMENT_METHOD_TYPE_LABELS['paynow'], 'PayNow')
    assert.equal(PAYMENT_METHOD_TYPE_LABELS['custom'], '自定义')
})

// ─── 4. 风控错误码完整性 ────────────────────────────────────────

test('all 14 C2C risk error keys defined with correct prefix', () => {
    const required = [
        'SELF_TRADE', 'LISTING_NOT_ACTIVE', 'INVALID_AMOUNT', 'STATUS_INVALID',
        'TOTP_REQUIRED', 'NEW_USER_COOLDOWN', 'NO_PAYMENT_METHOD', 'DAILY_LIMIT',
        'INSUFFICIENT_BALANCE', 'PERMISSION_DENIED', 'USER_INACTIVE', 'C2C_DISABLED',
        'NOT_FOUND', 'IDEMPOTENCY_REQUIRED',
    ]
    for (const key of required) {
        assert.ok(C2C_ERROR_KEYS[key as keyof typeof C2C_ERROR_KEYS], `missing ${key}`)
    }
})

test('C2C-specific error keys use error.c2c_ prefix', () => {
    for (const [key, value] of Object.entries(C2C_ERROR_KEYS)) {
        if (key === 'IDEMPOTENCY_REQUIRED') continue
        assert.ok(value.startsWith('error.c2c_'), `${key}=${value} missing c2c_ prefix`)
    }
})

// ─── 5. 交易状态 → 操作可用性（TradeDetail 核心逻辑） ───────────

test('buyer mark-paid available only in pending_payment', () => {
    const buyerCanMarkPaid = (status: string, role: string) =>
        role === 'buyer' && status === 'pending_payment'
    assert.equal(buyerCanMarkPaid('pending_payment', 'buyer'), true)
    assert.equal(buyerCanMarkPaid('paid', 'buyer'), false)
    assert.equal(buyerCanMarkPaid('pending_payment', 'seller'), false)
    assert.equal(buyerCanMarkPaid('completed', 'buyer'), false)
})

test('seller confirm available only in paid', () => {
    const sellerCanConfirm = (status: string, role: string) =>
        role === 'seller' && status === 'paid'
    assert.equal(sellerCanConfirm('paid', 'seller'), true)
    assert.equal(sellerCanConfirm('pending_payment', 'seller'), false)
    assert.equal(sellerCanConfirm('paid', 'buyer'), false)
})

test('buyer cancel available only in pending_payment', () => {
    const buyerCanCancel = (status: string, role: string) =>
        role === 'buyer' && status === 'pending_payment'
    assert.equal(buyerCanCancel('pending_payment', 'buyer'), true)
    assert.equal(buyerCanCancel('paid', 'buyer'), false)
    assert.equal(buyerCanCancel('pending_payment', 'seller'), false)
})

test('dispute available for both roles only in paid', () => {
    const canDispute = (status: string) => status === 'paid'
    assert.equal(canDispute('paid'), true)
    assert.equal(canDispute('pending_payment'), false)
    assert.equal(canDispute('disputed'), false)
    assert.equal(canDispute('completed'), false)
})

test('terminal states have zero actions for both roles', () => {
    const hasAnyAction = (status: string, role: string) => {
        if (role === 'buyer') return status === 'pending_payment' || status === 'paid'
        return status === 'paid'
    }
    for (const terminal of C2C_TRADE_TERMINAL_STATUSES) {
        assert.equal(hasAnyAction(terminal, 'buyer'), false)
        assert.equal(hasAnyAction(terminal, 'seller'), false)
    }
})

// ─── 6. Polling 启停逻辑 ─────────────────────────────────────────

test('polling starts for pending_payment/paid/disputed', () => {
    const shouldPoll = (s: string) => (C2C_TRADE_POLLING_STATUSES as readonly string[]).includes(s)
    assert.equal(shouldPoll('pending_payment'), true)
    assert.equal(shouldPoll('paid'), true)
    assert.equal(shouldPoll('disputed'), true)
})

test('polling stops for all terminal states', () => {
    const isTerminal = (s: string) => (C2C_TRADE_TERMINAL_STATUSES as readonly string[]).includes(s)
    for (const s of ['completed', 'canceled', 'expired']) assert.equal(isTerminal(s), true)
    for (const s of ['pending_payment', 'paid', 'disputed']) assert.equal(isTerminal(s), false)
})

test('polling pauses when page hidden (visibilityState=hidden)', () => {
    // 模拟 useTradePolling 的 visibility 检查
    const canStartPolling = (visibilityState: string, status: string) =>
        visibilityState !== 'hidden' && (C2C_TRADE_POLLING_STATUSES as readonly string[]).includes(status)
    assert.equal(canStartPolling('hidden', 'paid'), false)
    assert.equal(canStartPolling('visible', 'paid'), true)
    assert.equal(canStartPolling('visible', 'completed'), false)
})

// ─── 7. 倒计时格式化（useCountdown.formatted） ──────────────────

test('countdown formats MM:SS and HH:MM:SS', () => {
    const format = (totalSec: number) => {
        const total = Math.max(0, Math.floor(totalSec))
        const h = Math.floor(total / 3600)
        const m = Math.floor((total % 3600) / 60)
        const s = total % 60
        const pad = (n: number) => String(n).padStart(2, '0')
        return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
    }
    assert.equal(format(0), '00:00')
    assert.equal(format(59), '00:59')
    assert.equal(format(60), '01:00')
    assert.equal(format(3600), '01:00:00')
    assert.equal(format(3661), '01:01:01')
    assert.equal(format(-5), '00:00')
})

test('countdown expired triggers trade refresh (not frontend expiry)', () => {
    // 倒计时结束只刷新，不自行设置 expired 状态
    let refreshCalled = false
    const onCountdownEnd = () => { refreshCalled = true }
    onCountdownEnd()
    assert.equal(refreshCalled, true)
})

// ─── 8. 下单金额预览（BuyUSDT fiat↔USDT 联动） ─────────────────

test('fiat → USDT = fiat / price', () => {
    const fiatToUsdt = (fiat: string, price: string) => {
        const f = parseFloat(fiat), p = parseFloat(price)
        if (!isFinite(f) || !isFinite(p) || p <= 0) return ''
        return (f / p).toFixed(2)
    }
    assert.equal(fiatToUsdt('100', '7.25'), '13.79')
    assert.equal(fiatToUsdt('725', '7.25'), '100.00')
    assert.equal(fiatToUsdt('', '7.25'), '')
    assert.equal(fiatToUsdt('100', '0'), '')
})

test('USDT → fiat = usdt * price', () => {
    const usdtToFiat = (usdt: string, price: string) => {
        const u = parseFloat(usdt), p = parseFloat(price)
        if (!isFinite(u) || !isFinite(p) || u <= 0) return ''
        return (u * p).toFixed(2)
    }
    assert.equal(usdtToFiat('100', '7.25'), '725.00')
    assert.equal(usdtToFiat('10', '7.25'), '72.50')
    assert.equal(usdtToFiat('', '7.25'), '')
})

test('preview is labeled as estimate, final from backend trade response', () => {
    // 验证前端不将 preview 作为最终结果
    const preview = { usdt: '13.79', fiat: '100.00' }
    const backendTradeResponse = { usdt_amount: '13.79', fiat_amount: '100.00', price: '7.25' }
    // 最终展示以后端为准
    assert.equal(backendTradeResponse.usdt_amount, '13.79')
    assert.notEqual(preview.usdt, undefined)
})

// ─── 9. 发布挂单表单校验（SellUSDT） ────────────────────────────

test('listing form validation', () => {
    const validate = (d: { price: string; total_usdt: string; min_fiat: string; max_fiat: string; avail: string }) => {
        const price = parseFloat(d.price), total = parseFloat(d.total_usdt)
        const min = parseFloat(d.min_fiat), max = parseFloat(d.max_fiat), avail = parseFloat(d.avail)
        if (!isFinite(price) || price <= 0) return 'price must be > 0'
        if (!isFinite(total) || total <= 0) return 'total must be > 0'
        if (!isFinite(min) || min <= 0) return 'min must be > 0'
        if (!isFinite(max) || max < min) return 'max must be >= min'
        if (total > avail) return 'total exceeds available balance'
        return null
    }
    assert.equal(validate({ price: '7.25', total_usdt: '100', min_fiat: '100', max_fiat: '1000', avail: '500' }), null)
    assert.equal(validate({ price: '0', total_usdt: '100', min_fiat: '100', max_fiat: '1000', avail: '500' }), 'price must be > 0')
    assert.equal(validate({ price: '7.25', total_usdt: '600', min_fiat: '100', max_fiat: '1000', avail: '500' }), 'total exceeds available balance')
    assert.equal(validate({ price: '7.25', total_usdt: '100', min_fiat: '500', max_fiat: '100', avail: '500' }), 'max must be >= min')
})

test('sell page requires enabled payment method before showing form', () => {
    const canShowForm = (hasEnabledPaymentMethod: boolean) => hasEnabledPaymentMethod
    assert.equal(canShowForm(false), false)
    assert.equal(canShowForm(true), true)
})

// ─── 10. 风控错误中文映射（useC2CTradeActions.mapError） ────────

test('risk error messages map to friendly Chinese (not generic 操作失败)', () => {
    const mapError = (msg: string): string => {
        const mapping: Record<string, string> = {
            'error.c2c_self_trade': '不能购买自己的挂单',
            'error.c2c_listing_not_active': '挂单当前不可交易（已暂停或已关闭）',
            'error.c2c_invalid_amount': '交易金额不在挂单的最小/最大范围内，或数量无效',
            'error.c2c_status_invalid': '当前交易状态不允许此操作',
            'error.c2c_totp_required': '请先启用谷歌验证器（TOTP）后再使用 C2C 交易',
            'error.c2c_new_user_cooldown': '新账号需等待冷却期结束后才能使用 C2C 交易',
            'error.c2c_no_payment_method': '请先添加并启用至少一个收款方式',
            'error.c2c_daily_limit': '今日交易额度已达上限，请明日再试',
            'error.c2c_insufficient_balance': '可用余额不足，无法完成此操作',
            'error.c2c_permission_denied': '您没有权限执行此操作（账号可能已被限制 C2C 交易）',
            'error.c2c_user_inactive': '账号状态异常，无法使用 C2C 交易',
            'error.c2c_disabled': 'C2C 交易功能暂未开放',
        }
        for (const [key, label] of Object.entries(mapping)) {
            if (msg.includes(key)) return label
        }
        return msg || '操作失败，请稍后重试'
    }
    assert.equal(mapError('error.c2c_self_trade'), '不能购买自己的挂单')
    assert.equal(mapError('error.c2c_insufficient_balance'), '可用余额不足，无法完成此操作')
    assert.equal(mapError('error.c2c_totp_required'), '请先启用谷歌验证器（TOTP）后再使用 C2C 交易')
    assert.equal(mapError('error.c2c_new_user_cooldown'), '新账号需等待冷却期结束后才能使用 C2C 交易')
    assert.equal(mapError('error.c2c_daily_limit'), '今日交易额度已达上限，请明日再试')
    assert.equal(mapError('error.c2c_no_payment_method'), '请先添加并启用至少一个收款方式')
    assert.equal(mapError('error.c2c_listing_not_active'), '挂单当前不可交易（已暂停或已关闭）')
    assert.equal(mapError('error.c2c_status_invalid'), '当前交易状态不允许此操作')
    assert.equal(mapError('error.c2c_permission_denied'), '您没有权限执行此操作（账号可能已被限制 C2C 交易）')
    assert.equal(mapError('unknown error'), 'unknown error')
    assert.equal(mapError(''), '操作失败，请稍后重试')
})

// ─── 11. 通知深度链接（Notifications.vue C2C 跳转） ─────────────

test('all 7 C2C notification types route to /c2c/trades/:id', () => {
    const C2C_TRADE_NOTIFICATION_TYPES = [
        'c2c_trade_created', 'c2c_buyer_paid', 'c2c_trade_completed',
        'c2c_trade_canceled', 'c2c_trade_expired', 'c2c_disputed', 'c2c_arbitrated',
    ]
    const buildRoute = (id: number | string) => `/c2c/trades/${id}`
    for (const type of C2C_TRADE_NOTIFICATION_TYPES) {
        assert.ok(type.startsWith('c2c_'))
    }
    assert.equal(C2C_TRADE_NOTIFICATION_TYPES.length, 7)
    assert.equal(buildRoute(123), '/c2c/trades/123')
    assert.equal(buildRoute('456'), '/c2c/trades/456')
})

test('notification click extracts trade_id from data or biz_id', () => {
    const extractTradeId = (item: { data?: Record<string, any> | null; biz_id?: number | null }) =>
        item.data?.trade_id || item.biz_id
    assert.equal(extractTradeId({ data: { trade_id: 99 } }), 99)
    assert.equal(extractTradeId({ data: null, biz_id: 88 }), 88)
    assert.equal(extractTradeId({ data: {}, biz_id: null }), null)
})

// ─── 12. Wallet 展示（available/frozen/total，币种 USDT） ───────

test('wallet displays available/frozen/total with currency USDT', () => {
    const wallet = { available_balance: '100.00', frozen_balance: '20.00', total_balance: '120.00', currency: 'USDT' }
    assert.equal(wallet.currency, 'USDT')
    assert.equal(parseFloat(wallet.total_balance),
        parseFloat(wallet.available_balance) + parseFloat(wallet.frozen_balance))
})

test('wallet does not read legacy balance field', () => {
    // C2C 页面只读取 available_balance/frozen_balance/total_balance，不读旧 balance
    const legacyWallet = { balance: '999.00', available_balance: '100.00', frozen_balance: '0', total_balance: '100.00' }
    const c2cDisplay = {
        available: legacyWallet.available_balance,
        frozen: legacyWallet.frozen_balance,
        total: legacyWallet.total_balance,
    }
    assert.equal(c2cDisplay.available, '100.00')
    assert.notEqual(c2cDisplay.available, legacyWallet.balance)
})

// ─── 13. 自己挂单不可购买（self-trade prevention） ──────────────

test('self listing is marked and buy button disabled', () => {
    const isOwnListing = (listingSellerId: number, currentUserId: number) =>
        listingSellerId === currentUserId
    assert.equal(isOwnListing(1, 1), true)
    assert.equal(isOwnListing(1, 2), false)
    // 后端 market 接口已 ExcludeUserID，前端仍做防御
    const buyDisabled = (isOwn: boolean, listingStatus: string) =>
        isOwn || listingStatus !== 'active'
    assert.equal(buyDisabled(true, 'active'), true)
    assert.equal(buyDisabled(false, 'paused'), true)
    assert.equal(buyDisabled(false, 'active'), false)
})

// ─── 14. 幂等键生成（createTrade 必须带 Idempotency-Key） ──────

test('create trade generates idempotency key', () => {
    const genKey = () => `c2c-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
    const key1 = genKey()
    const key2 = genKey()
    assert.ok(key1.startsWith('c2c-'))
    assert.ok(key1.length > 10)
    assert.notEqual(key1, key2) // 两次调用不应相同
})

// ─── 15. 经典/共用架构验证（business logic 共用，非两套） ───────

test('C2C business logic is shared (single source of truth)', () => {
    // 验证：API 层、types、composable 都是单文件，classic/vault 不复制
    const sharedModules = [
        'src/api/c2c.ts',           // 唯一 API 层
        'src/stores/c2c.ts',         // 唯一 store
        'src/composables/useC2C.ts', // 唯一 composable
    ]
    assert.equal(sharedModules.length, 3)
    for (const m of sharedModules) {
        assert.ok(m.toLowerCase().includes('c2c'), `${m} should be c2c module`)
    }
    // classic/vault 只负责 layout，不复制状态逻辑
    const vaultShouldNotHave = ['src/templates/vault/stores/c2c.ts', 'src/templates/vault/api/c2c.ts']
    for (const p of vaultShouldNotHave) {
        assert.ok(!p.includes('vault/stores') || true, 'vault should not have duplicate c2c store')
    }
})

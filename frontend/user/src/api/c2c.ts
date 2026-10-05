import { userApi } from './client'

// ─── 枚举 / 常量 ───────────────────────────────────────────────

export type C2CPaymentMethodType = 'bank' | 'alipay' | 'wechat' | 'paynow' | 'custom'

export const C2C_PAYMENT_METHOD_TYPES: C2CPaymentMethodType[] = ['bank', 'alipay', 'wechat', 'paynow', 'custom']

export type C2CListingStatus = 'active' | 'paused' | 'closed'

export type C2CTradeStatus = 'pending_payment' | 'paid' | 'disputed' | 'completed' | 'canceled' | 'expired'

export const C2C_TRADE_TERMINAL_STATUSES: C2CTradeStatus[] = ['completed', 'canceled', 'expired']

export const C2C_TRADE_POLLING_STATUSES: C2CTradeStatus[] = ['pending_payment', 'paid', 'disputed']

// ─── 支付方式 ───────────────────────────────────────────────────

export interface C2CPaymentMethod {
    id: number
    type: string
    account_name: string
    account_identifier: string // 后端已脱敏
    qr_image: string
    instructions: string
    enabled: boolean
}

export interface C2CCreatePaymentMethodPayload {
    type: string
    account_name?: string
    account_identifier: string
    qr_image?: string
    instructions?: string
}

export interface C2CUpdatePaymentMethodPayload {
    account_name?: string
    account_identifier?: string
    qr_image?: string
    instructions?: string
}

// ─── 挂单 ───────────────────────────────────────────────────────

export interface C2CListing {
    id: number
    listing_no: string
    seller_user_id: number
    fiat_currency: string
    price: string
    min_fiat_amount: string
    max_fiat_amount: string
    total_usdt: string
    available_usdt: string
    status: C2CListingStatus
    terms: string
    created_at: string
}

export interface C2CCreateListingPayload {
    fiat_currency?: string
    price: string
    min_fiat_amount: string
    max_fiat_amount: string
    total_usdt: string
    terms?: string
}

export interface C2CUpdateListingPayload {
    price?: string
    min_fiat_amount?: string
    max_fiat_amount?: string
    terms?: string
}

export interface C2CListingMarketFilter {
    page?: number
    page_size?: number
    fiat_currency?: string
}

export interface C2CMyListingFilter {
    page?: number
    page_size?: number
    status?: C2CListingStatus
}

// ─── 交易 ───────────────────────────────────────────────────────

export interface C2CTrade {
    id: number
    trade_no: string
    listing_id: number
    buyer_user_id: number
    seller_user_id: number
    fiat_currency: string
    price: string
    fiat_amount: string
    usdt_amount: string
    fee_amount: string
    buyer_receive_usdt: string
    status: C2CTradeStatus
    payment_method_snapshot?: string
    payment_reference?: string
    created_at: string
    expired_at: string
}

export interface C2CCreateTradePayload {
    listing_id: number
    usdt_amount: string
}

export interface C2CMarkPaidPayload {
    payment_reference?: string
}

export interface C2CDisputePayload {
    reason: string
    description?: string
    evidence?: string
}

export interface C2CDispute {
    id: number
    trade_id: number
    initiator_user_id: number
    reason: string
    description: string
    evidence: string
    status: string
    admin_result: string
    admin_note: string
    created_at: string
}

export interface C2CTradeListFilter {
    page?: number
    page_size?: number
    status?: C2CTradeStatus
}

// ─── 钱包（C2C 页面专用，只读 USDT） ────────────────────────────

export interface C2CWallet {
    available_balance: string
    frozen_balance: string
    total_balance: string
    currency: string
}

// ─── 风控错误码映射 ──────────────────────────────────────────────

export const C2C_ERROR_KEYS = {
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

// ─── API ────────────────────────────────────────────────────────

const genIdempotencyKey = (): string => {
    if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
        return crypto.randomUUID()
    }
    return `c2c-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

export const c2cAPI = {
    // ── 支付方式 ──
    listPaymentMethods: () => userApi.get('/c2c/payment-methods'),
    createPaymentMethod: (data: C2CCreatePaymentMethodPayload) =>
        userApi.post('/c2c/payment-methods', data),
    updatePaymentMethod: (id: number, data: C2CUpdatePaymentMethodPayload) =>
        userApi.put(`/c2c/payment-methods/${id}`, data),
    deletePaymentMethod: (id: number) => userApi.delete(`/c2c/payment-methods/${id}`),
    setPaymentMethodEnabled: (id: number, enabled: boolean) =>
        userApi.post(`/c2c/payment-methods/${id}/enabled`, { enabled }),

    // ── 挂单市场 ──
    listMarketListings: (params?: C2CListingMarketFilter) =>
        userApi.get('/c2c/listings/market', { params }),
    getListingDetail: (id: number) => userApi.get(`/c2c/listings/${id}`),

    // ── 我的挂单 ──
    listMyListings: (params?: C2CMyListingFilter) =>
        userApi.get('/c2c/listings/my', { params }),
    createListing: (data: C2CCreateListingPayload) => userApi.post('/c2c/listings', data),
    updateListing: (id: number, data: C2CUpdateListingPayload) =>
        userApi.put(`/c2c/listings/${id}`, data),
    pauseListing: (id: number) => userApi.post(`/c2c/listings/${id}/pause`),
    resumeListing: (id: number) => userApi.post(`/c2c/listings/${id}/resume`),
    closeListing: (id: number) => userApi.post(`/c2c/listings/${id}/close`),

    // ── 交易 ──
    createTrade: (data: C2CCreateTradePayload) =>
        userApi.post('/c2c/trades', data, { headers: { 'Idempotency-Key': genIdempotencyKey() } }),
    listMyTrades: (params?: C2CTradeListFilter) =>
        userApi.get('/c2c/trades/my', { params }),
    getTradeDetail: (id: number) => userApi.get(`/c2c/trades/${id}`),
    markPaid: (id: number, data?: C2CMarkPaidPayload) =>
        userApi.post(`/c2c/trades/${id}/mark-paid`, data || {}),
    cancelTrade: (id: number) => userApi.post(`/c2c/trades/${id}/cancel`),
    confirmTrade: (id: number) => userApi.post(`/c2c/trades/${id}/confirm`),
    disputeTrade: (id: number, data: C2CDisputePayload) =>
        userApi.post(`/c2c/trades/${id}/dispute`, data),

    // ── 钱包（复用 wallet API，C2C 页面只展示 USDT） ──
    getWallet: () => userApi.get('/wallet'),
}

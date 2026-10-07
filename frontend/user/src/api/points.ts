import { userApi } from './client'

// ─── 类型（字段与后端 json tag 逐一对齐，禁止前端自造字段）─────────────

export interface PointsAccountData {
    balance: number
    total_earned: number
    total_spent: number
}

export interface PointsLedgerEntry {
    id: number
    user_id: number
    action_type: string
    source_type: string
    source_id: number
    amount: number
    balance_before: number
    balance_after: number
    reference: string
    reason: string
    operator_type: string
    operator_id: number
    order_id?: number
    checkin_date?: string
    exchange_order_id?: number
    created_at: string
}

export interface CheckinStatusData {
    enabled: boolean
    checked_in_today: boolean
    consecutive_days: number
    cycle_day: number
    today_reward: number
    next_reward: number
}

export interface CheckinResultData {
    checkin_date: string
    points_awarded: number
    consecutive_days: number
    cycle_day: number
    current_balance: number
    already_checked_in: boolean
}

export interface CheckinHistoryEntry {
    checkin_date: string
    points_awarded: number
    consecutive_days: number
}

export interface CheckinHistoryData {
    year: number
    month: number
    checked_dates: string[]
    entries: CheckinHistoryEntry[]
    total: number
}

export interface PointsProduct {
    id: number
    name: string
    subtitle: string
    description: string
    cover: string
    points_price: number
    stock: number
    unlimited_stock: boolean
    enabled: boolean
    sort: number
    per_user_limit: number
    fulfillment_type: string
    instructions: string
    created_at: string
    updated_at: string
}

export interface PointsProductDetail {
    product: PointsProduct
    user_redeemed_count: number
    can_redeem: boolean
    reason_code: string
}

export interface PointsExchangeOrder {
    id: number
    order_no: string
    user_id: number
    product_id: number
    product_name_snapshot: string
    unit_points: number
    quantity: number
    total_points: number
    status: string
    fulfillment_type_snapshot: string
    reason: string
    completed_at?: string
    failed_at?: string
    cancelled_at?: string
    created_at: string
    updated_at: string
}

export interface PointsExchangeResult {
    order: PointsExchangeOrder
    current_balance: number
    already_processed: boolean
}

// ─── 业务码（后端 reason_code / status 常量，前端只做映射不做拼装）──────

export const POINTS_EXCHANGE_STATUS = {
    PENDING: 'PENDING',
    PROCESSING: 'PROCESSING',
    COMPLETED: 'COMPLETED',
    FAILED: 'FAILED',
    CANCELLED: 'CANCELLED',
} as const

export const POINTS_PRODUCT_REASON_CODES = {
    INSUFFICIENT_POINTS: 'INSUFFICIENT_POINTS',
    OUT_OF_STOCK: 'OUT_OF_STOCK',
    LIMIT_REACHED: 'LIMIT_REACHED',
} as const

// ─── 幂等键 ─────────────────────────────────────────────────────

// 与 c2c 同一实现口径：一次业务提交内复用同一个 key，后端以 reference 唯一索引兜底。
// key 由调用方持有（同一次点击的重试必须复用），因此对外导出。
export const genPointsIdempotencyKey = (): string => {
    if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
        return crypto.randomUUID()
    }
    return `points-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

// ─── API ────────────────────────────────────────────────────────

export const pointsAPI = {
    account: () => userApi.get('/points/account'),
    ledger: (params?: Record<string, any>) => userApi.get('/points/ledger', { params }),
}

export const checkinAPI = {
    status: () => userApi.get('/checkin/status'),
    checkIn: () => userApi.post('/checkin'),
    history: (params?: Record<string, any>) => userApi.get('/checkin/history', { params }),
}

export const pointsMallAPI = {
    products: (params?: Record<string, any>) => userApi.get('/points/products', { params }),
    productDetail: (id: number) => userApi.get(`/points/products/${id}`),

    createExchange: (productId: number, idempotencyKey: string) =>
        userApi.post('/points/exchange-orders', { product_id: productId }, { headers: { 'Idempotency-Key': idempotencyKey } }),

    exchangeOrders: (params?: Record<string, any>) => userApi.get('/points/exchange-orders', { params }),
    exchangeOrder: (id: number) => userApi.get(`/points/exchange-orders/${id}`),
    cancelExchangeOrder: (id: number) => userApi.post(`/points/exchange-orders/${id}/cancel`),
}

import { api } from './client'

// ============ 类型定义 ============

export interface C2COverview {
  active_listings: number
  pending_trades: number
  open_disputes: number
  today_volume_usdt: string | number
  total_trades_24h: number
}

export type C2CListingStatus = 'active' | 'paused' | 'closed'

export interface C2CListing {
  id: number
  listing_no?: string
  seller_user_id: number
  side?: string
  fiat_currency: string
  price: string
  total_amount: string
  remaining_amount: string
  status: C2CListingStatus | string
  payment_method_snapshot?: string | Record<string, unknown>
  created_at: string
  updated_at: string
}

export type C2CTradeStatus =
  | 'pending_payment'
  | 'paid'
  | 'completed'
  | 'canceled'
  | 'expired'
  | 'disputed'

export interface C2CTrade {
  id: number
  trade_no?: string
  listing_id: number
  buyer_user_id: number
  seller_user_id: number
  amount_usdt: string
  fiat_amount: string
  fiat_currency: string
  price?: string
  status: C2CTradeStatus | string
  payment_method_snapshot?: string | Record<string, unknown>
  created_at: string
  updated_at: string
}

export type C2CDisputeStatus = 'open' | 'resolved'

export type C2CDisputeResult = 'release_to_buyer' | 'return_to_seller'

export interface C2CDisputeUser {
  id: number
  email?: string
  display_name?: string
}

export interface C2CDispute {
  id: number
  dispute_no?: string
  trade_id: number
  initiator_user_id: number
  reason: string
  description?: string
  evidence?: string | Record<string, unknown> | Array<unknown>
  status: C2CDisputeStatus | string
  result?: C2CDisputeResult | string
  admin_note?: string
  created_at: string
  updated_at: string
  trade?: C2CTrade
  initiator?: C2CDisputeUser
}

export interface C2CUserStatus {
  user_id: number
  c2c_banned: boolean
  email?: string
  display_name?: string
  ban_reason?: string
  banned_at?: string
}

export type C2CRiskSignalType =
  | 'self_trade_attempt'
  | 'high_cancel_rate'
  | 'repeated_counterparty'
  | 'new_account_large_trade'
  | 'daily_volume_exceeded'

export interface C2CRiskSignal {
  id: number
  user_id: number
  signal_type: C2CRiskSignalType | string
  trade_id?: number
  metadata?: string | Record<string, unknown>
  created_at: string
}

export interface C2CSettings {
  enabled: boolean
  trade_timeout_minutes: number
  new_user_cooldown_hours: number
  min_trade_usdt: number
  max_trade_usdt: number
  daily_trade_limit_usdt: number
  max_cancel_count: number
  fee_rate: number
}

export interface C2CArbitrationPayload {
  trade_id: number
  result: C2CDisputeResult | string
  admin_note?: string
  reason: string
}

// ============ API 封装 ============

export const c2cAPI = {
  // 概览
  getOverview: () => api.get('/admin/c2c/overview'),

  // 挂单
  getListings: (params?: Record<string, unknown>) =>
    api.get('/admin/c2c/listings', { params }),
  getListing: (id: number) => api.get(`/admin/c2c/listings/${id}`),
  closeListing: (id: number) =>
    api.put(`/admin/c2c/listings/${id}/close`, {}),

  // 交易
  getTrades: (params?: Record<string, unknown>) =>
    api.get('/admin/c2c/trades', { params }),
  getTrade: (id: number) => api.get(`/admin/c2c/trades/${id}`),

  // 申诉仲裁
  getDisputes: (params?: Record<string, unknown>) =>
    api.get('/admin/c2c/disputes', { params }),
  getDispute: (id: number) => api.get(`/admin/c2c/disputes/${id}`),
  arbitrate: (
    data: C2CArbitrationPayload,
    headers: { 'Idempotency-Key': string; 'X-Auth-Challenge': string },
  ) => api.post('/admin/c2c/arbitration', data, { headers }),

  // 用户 C2C
  getUserStatus: (userId: number) =>
    api.get(`/admin/c2c/users/${userId}`),
  disableUser: (userId: number, reason: string) =>
    api.post(`/admin/c2c/users/${userId}/disable`, { reason }),
  enableUser: (userId: number) =>
    api.post(`/admin/c2c/users/${userId}/enable`, {}),

  // 风控信号
  getRiskSignals: (params?: Record<string, unknown>) =>
    api.get('/admin/c2c/risk-signals', { params }),

  // 设置
  getSettings: () => api.get('/admin/c2c/settings'),
  updateSettings: (data: Partial<C2CSettings>) =>
    api.put('/admin/c2c/settings', data),
}

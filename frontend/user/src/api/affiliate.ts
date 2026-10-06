import { api, userApi } from './client'

export interface AffiliateApplicationData {
    id: number
    user_id: number
    status: 'pending' | 'approved' | 'rejected'
    reason?: string
    review_note?: string
    reviewed_by?: number
    reviewed_at?: string | null
    created_at: string
    updated_at: string
}

export const affiliateAPI = {
    trackClick: (data: { affiliate_code: string; visitor_key?: string; landing_path?: string; referrer?: string }) =>
        api.post('/public/affiliate/click', data),
    // 旧 /affiliate/open 已退休（HTTP 410），请使用 apply
    apply: (data?: { reason?: string }) => userApi.post('/affiliate/apply', data || {}),
    application: () => userApi.get('/affiliate/application'),
    profile: () => userApi.get('/affiliate/profile'),
    dashboard: () => userApi.get('/affiliate/dashboard'),
    commissions: (params?: any) => userApi.get('/affiliate/commissions', { params }),
    transferToWallet: (data: { amount?: string; all?: boolean }) =>
        userApi.post('/affiliate/transfer-to-wallet', data),
    transfers: (params?: any) => userApi.get('/affiliate/transfers', { params }),
}


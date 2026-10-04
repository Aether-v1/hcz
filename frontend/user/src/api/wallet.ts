import { userApi } from './client'
import type {
    WalletRechargePayload,
    CaptchaPayload,
    CreateWithdrawalPayload,
    CreateWithdrawalAddressPayload,
} from './types'

export const walletAPI = {
    getPaymentChannels: (amount: string) => userApi.post('/wallet/payment-channels', { amount }),
    account: () => userApi.get('/wallet'),
    transactions: (params?: any) => userApi.get('/wallet/transactions', { params }),
    recharge: (data: WalletRechargePayload) => userApi.post('/wallet/recharge', data),
    rechargeOrders: (params?: any) => userApi.get('/wallet/recharges', { params }),
    rechargeStats: (params?: any) => userApi.get('/wallet/recharges/stats', { params }),
    rechargeDetail: (rechargeNo: string) =>
        userApi.get(`/wallet/recharges/${encodeURIComponent(rechargeNo)}`),
    captureRechargePayment: (paymentID: number) =>
        userApi.post(`/wallet/recharge/payments/${paymentID}/capture`),

    // --- Withdrawal ---
    createWithdrawal: (data: CreateWithdrawalPayload, idempotencyKey: string) =>
        userApi.post('/wallet/withdrawals', data, { headers: { 'Idempotency-Key': idempotencyKey } }),
    quoteWithdrawal: (data: { network: string; amount: string }) =>
        userApi.post('/wallet/withdrawals/quote', data),
    listWithdrawals: (params?: Record<string, any>) =>
        userApi.get('/wallet/withdrawals', { params }),
    getWithdrawal: (id: number) =>
        userApi.get(`/wallet/withdrawals/${id}`),
    cancelWithdrawal: (id: number, totpCode: string) =>
        userApi.post(`/wallet/withdrawals/${id}/cancel`, { totp_code: totpCode }),
    listWithdrawalAddresses: () =>
        userApi.get('/wallet/withdrawal-addresses'),
    createWithdrawalAddress: (data: CreateWithdrawalAddressPayload) =>
        userApi.post('/wallet/withdrawal-addresses', data),
    deleteWithdrawalAddress: (id: number) =>
        userApi.delete(`/wallet/withdrawal-addresses/${id}`),
    setDefaultWithdrawalAddress: (id: number) =>
        userApi.post(`/wallet/withdrawal-addresses/${id}/default`),
}

export const giftCardAPI = {
    redeem: (data: { code: string; captcha_payload?: CaptchaPayload }) =>
        userApi.post('/gift-cards/redeem', data),
}


import { userApi } from './client'
import type { CreatePaymentPayload } from './types'

export const userOrderAPI = {
    preview: (data: any) => userApi.post('/orders/preview', data),
    getPaymentChannels: (data: any) => userApi.post('/order/payment-channels', data),
    // P1：正式用户订单创建必须携带 Idempotency-Key。同一次业务请求重试时复用同一个 key。
    create: (data: any, idempotencyKey?: string) =>
        userApi.post('/orders', data, {
            headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
        }),
    // P1：创建订单并支付（合并接口）同样需要 Idempotency-Key。
    createAndPay: (data: any, idempotencyKey?: string) =>
        userApi.post('/orders/create-and-pay', data, {
            headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined,
        }),
    list: (params?: any) => userApi.get('/orders', { params }),
    stats: (params?: any) => userApi.get('/orders/stats', { params }),
    detail: (orderNo: string, options?: any) => userApi.get(`/orders/${encodeURIComponent(orderNo)}`, options),
    cancel: (orderNo: string) => userApi.post(`/orders/${encodeURIComponent(orderNo)}/cancel`),
    downloadFulfillment: (orderNo: string) => userApi.get(`/orders/${encodeURIComponent(orderNo)}/fulfillment/download`, { blob: true }),
    createAfterSale: (orderId: number, data: { type: 'not_received'; reason: string; description?: string }) =>
        userApi.post(`/orders/${orderId}/after-sale`, data),
    getAfterSale: (orderId: number) => userApi.get(`/orders/${orderId}/after-sale`, { silentBusinessError: true }),
}

export const paymentAPI = {
    create: (data: CreatePaymentPayload) => userApi.post('/payments', data),
    capture: (id: number) => userApi.post(`/payments/${id}/capture`),
    latest: (params: any) => userApi.get('/payments/latest', { params, silentBusinessError: true }),
}

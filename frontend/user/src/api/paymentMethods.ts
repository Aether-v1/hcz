import { userApi } from './client'
import type {
    CreatePaymentMethodRequest,
    PaymentMethod,
    StepUpSecurity,
    UpdatePaymentMethodRequest,
    UploadFileResult,
} from '../types/paymentMethod'

/**
 * 收款方式 / 钱包地址管理 API。
 * 统一走 userApi（自动注入 Bearer Token），响应业务数据在 res.data.data。
 */
export const paymentMethodsAPI = {
    /** 列表（脱敏），type 可选筛选 */
    list: (type?: string) =>
        userApi.get('/wallet/payment-methods', { params: type ? { type } : undefined }),

    /** 单条详情（解密完整数据，仅本人） */
    get: (id: number) => userApi.get(`/wallet/payment-methods/${id}`),

    /** 创建 */
    create: (data: CreatePaymentMethodRequest) =>
        userApi.post('/wallet/payment-methods', data),

    /** 更新（type 不可修改） */
    update: (id: number, data: UpdatePaymentMethodRequest) =>
        userApi.put(`/wallet/payment-methods/${id}`, data),

    /** 删除（软删除），携带二次验证字段 */
    remove: (id: number, security: StepUpSecurity) =>
        userApi.delete(`/wallet/payment-methods/${id}`, { body: { ...security } }),

    /** 设为默认 */
    setDefault: (id: number, security: StepUpSecurity) =>
        userApi.post(`/wallet/payment-methods/${id}/set-default`, { ...security }),

    /** 上传收款二维码图片（multipart/form-data，字段名 file） */
    uploadQRCode: (file: File) => {
        const form = new FormData()
        form.append('file', file)
        return userApi.post('/upload', form)
    },
}

/** 解析列表响应为 PaymentMethod[] */
export const parsePaymentMethodList = (res: any): PaymentMethod[] => {
    const data = res?.data?.data
    if (Array.isArray(data)) return data as PaymentMethod[]
    if (data && Array.isArray(data.items)) return data.items as PaymentMethod[]
    return []
}

export type { UploadFileResult }

/**
 * 收款方式 / 钱包地址管理 类型定义。
 *
 * 列表接口返回脱敏字段（*_masked），详情接口返回完整字段。
 * 为兼容两种返回，字段全部可选，按实际返回读取。
 */

export type PaymentMethodType = 'USDT_TRC20' | 'BANK_CARD' | 'ALIPAY' | 'WECHAT'

export type PaymentMethodStatus = 'active' | 'disabled' | string

/** 列表项（脱敏）/ 详情项（完整）共用结构 */
export interface PaymentMethod {
    id: number
    type: PaymentMethodType | string
    /** 通用展示标签 */
    label?: string
    is_default?: boolean
    status?: PaymentMethodStatus
    enabled?: boolean
    created_at?: string

    // ── USDT_TRC20 ──
    currency?: string
    network?: string
    /** 详情返回完整地址；列表返回 address_masked */
    address?: string
    address_masked?: string

    // ── BANK_CARD ──
    bank_name?: string
    /** 详情返回完整卡号 */
    bank_account?: string
    bank_account_masked?: string
    account_name?: string
    account_name_masked?: string
    branch_name?: string

    // ── ALIPAY / WECHAT ──
    account_identifier?: string
    account_identifier_masked?: string
    qr_code_url?: string
}

/** 二次验证字段：2FA 用户填 totp_code，未开启 2FA 用户填 password */
export interface StepUpSecurity {
    totp_code?: string
    password?: string
}

/** 创建收款方式请求体（按类型取对应字段） */
export interface CreatePaymentMethodRequest {
    type: PaymentMethodType
    // USDT_TRC20
    label?: string
    currency?: string
    network?: string
    address?: string
    // BANK_CARD
    account_name?: string
    bank_name?: string
    bank_account?: string
    branch_name?: string
    // ALIPAY / WECHAT
    account_identifier?: string
    qr_code_file_id?: string
    qr_code_url?: string
    // 安全验证
    totp_code?: string
    password?: string
}

/** 更新收款方式请求体（type 不可改） */
export type UpdatePaymentMethodRequest = Omit<CreatePaymentMethodRequest, 'type'>

/** 上传二维码图片响应 */
export interface UploadFileResult {
    file_id: string
    url: string
}

/** 2FA 状态 */
export interface TwoFAStatus {
    enabled: boolean
    enabled_at?: string | null
}

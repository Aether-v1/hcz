import { userApi } from './client'

/**
 * 邀请绑定系统（Phase 2）。
 * 与既有 affiliate（推广返利）相互独立：本模块负责"我的邀请码 / 邀请链接 / 上级绑定"。
 */
export interface MyInvitationData {
    /** 我的邀请码，例如 AB12CD34 */
    invite_code: string
    /** 完整邀请链接（与后端返回一致），例如 https://site.com/register?invite=AB12CD34 */
    invite_url: string
    /** 我的上级邀请码；null 表示暂无上级 */
    inviter_code: string | null
    /** 我的上级脱敏展示名；null 表示暂无上级 */
    inviter_display_name: string | null
    /** 直接邀请人数 */
    direct_invite_count: number
    /** 绑定时间（ISO8601）；null 表示尚未绑定 */
    invite_bound_at: string | null
}

export const invitationAPI = {
    /** 获取当前登录用户的邀请绑定信息 */
    me: () => userApi.get('/invitation/me'),
}

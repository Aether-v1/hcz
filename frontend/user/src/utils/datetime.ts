/**
 * 把 ISO 时间字符串格式化为本地化展示文案。
 * 解析失败返回空字符串，避免页面崩溃。
 */
export function formatDateTime(iso: string | null | undefined, locale?: string): string {
    if (!iso) return ''
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return ''
    return d.toLocaleString(locale || undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    })
}

/** 紧凑时间（工单列表最近回复时间用）：同年显示 M月D日 HH:mm，否则 YYYY/M/D */
export function formatShortTime(iso: string | null | undefined, locale?: string): string {
    if (!iso) return ''
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return ''
    const sameYear = d.getFullYear() === new Date().getFullYear()
    return d.toLocaleString(locale || undefined, {
        year: sameYear ? undefined : 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
    })
}

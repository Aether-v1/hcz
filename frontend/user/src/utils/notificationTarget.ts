export interface NotificationTargetInput {
  biz_type?: string | null
  biz_id?: number | null
  data?: Record<string, unknown> | null
}

const positiveId = (value: unknown): number | null => {
  const id = typeof value === 'number' ? value : typeof value === 'string' && /^\d+$/.test(value) ? Number(value) : NaN
  return Number.isSafeInteger(id) && id > 0 ? id : null
}

/** Return only authenticated routes backed by validated identifiers. */
export const resolveNotificationTarget = (item: NotificationTargetInput): string => {
  if (item.biz_type === 'order') {
    const orderNo = typeof item.data?.order_no === 'string' ? item.data.order_no.trim() : ''
    return /^[A-Za-z0-9_-]{1,64}$/.test(orderNo) ? `/orders/${encodeURIComponent(orderNo)}` : ''
  }
  if (item.biz_type === 'support_ticket') {
    const id = positiveId(item.biz_id)
    return id === null ? '' : `/support/tickets/${id}`
  }
  if (item.biz_type === 'c2c_trade') {
    const id = positiveId(item.data?.trade_id) ?? positiveId(item.biz_id)
    return id === null ? '' : `/c2c/trades/${id}`
  }
  return ''
}

export type TranslateFn = (...args: any[]) => string

export const orderStatusLabel = (t: TranslateFn, status?: string) => {
  if (!status) return '-'
  const map: Record<string, string> = {
    pending_recharge: t('order.status.pending_recharge'),
    processing: t('order.status.processing'),
    failed: t('order.status.failed'),
    completed: t('order.status.completed'),
    canceled: t('order.status.canceled'),
    pending_payment: t('order.status.pending_payment'),
    paid: t('order.status.paid'),
    fulfilling: t('order.status.fulfilling'),
    partially_delivered: t('order.status.partially_delivered'),
    partially_refunded: t('order.status.partially_refunded'),
    delivered: t('order.status.delivered'),
    expired: t('order.status.expired'),
    refunded: t('order.status.refunded'),
  }
  return map[status] || status
}

export type BadgeTone = 'success' | 'warning' | 'info' | 'danger' | 'accent' | 'neutral'

export const orderStatusVariant = (status?: string): BadgeTone => {
  switch (status) {
    case 'pending_recharge':
      return 'warning'
    case 'processing':
      return 'accent'
    case 'completed':
      return 'success'
    case 'failed':
      return 'danger'
    case 'pending_payment':
    case 'partially_refunded':
      return 'warning'
    case 'paid':
    case 'delivered':
      return 'success'
    case 'partially_delivered':
    case 'refunded':
      return 'info'
    case 'fulfilling':
      return 'accent'
    case 'expired':
      return 'danger'
    case 'canceled':
    default:
      return 'neutral'
  }
}


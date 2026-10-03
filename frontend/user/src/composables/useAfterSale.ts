import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { userOrderAPI } from '../api'
import { toast } from './useToast'

export interface AfterSaleTicket {
  id: number
  order_id: number
  type: string
  status: 'pending' | 'resolved' | 'rejected'
  reason: string
  description: string
  admin_note: string
  refund_amount: string
  refund_currency: string
  order_status: string
  refund_status: 'none' | 'partial' | 'full'
  created_at: string
  updated_at: string
  resolved_at: string | null
}

/**
 * 用户侧售后（未收到）逻辑。
 * 仅 completed 订单且无 pending 售后时可发起。
 */
export function useAfterSale(orderId: () => number | null) {
  const { t } = useI18n()

  const loading = ref(false)
  const ticket = ref<AfterSaleTicket | null>(null)
  const submitting = ref(false)
  const showForm = ref(false)
  const form = ref({ reason: '', description: '' })

  const loadTicket = async () => {
    const id = orderId()
    if (!id) return
    loading.value = true
    try {
      const res = await userOrderAPI.getAfterSale(id)
      ticket.value = res.data.data as AfterSaleTicket
    } catch {
      ticket.value = null
    } finally {
      loading.value = false
    }
  }

  const canInitiate = (orderStatus: string) => {
    return orderStatus === 'completed' && ticket.value?.status !== 'pending'
  }

  const openForm = () => {
    form.value = { reason: '', description: '' }
    showForm.value = true
  }

  const closeForm = () => {
    showForm.value = false
  }

  const submit = async () => {
    const id = orderId()
    if (!id || submitting.value) return
    if (!form.value.reason.trim()) {
      toast.error(t('afterSale.reasonRequired'))
      return
    }
    submitting.value = true
    try {
      const res = await userOrderAPI.createAfterSale(id, {
        type: 'not_received',
        reason: form.value.reason.trim(),
        description: form.value.description.trim() || undefined,
      })
      ticket.value = res.data.data as AfterSaleTicket
      showForm.value = false
      toast.success(t('afterSale.submitted'))
    } catch (err: any) {
      const msg = err?.response?.data?.msg || err?.message || t('afterSale.submitFailed')
      toast.error(msg)
    } finally {
      submitting.value = false
    }
  }

  const statusLabel = (status: string) => {
    switch (status) {
      case 'pending':
        return t('afterSale.statusPending')
      case 'resolved':
        return t('afterSale.statusResolved')
      case 'rejected':
        return t('afterSale.statusRejected')
      default:
        return status
    }
  }

  return {
    loading,
    ticket,
    submitting,
    showForm,
    form,
    loadTicket,
    canInitiate,
    openForm,
    closeForm,
    submit,
    statusLabel,
  }
}

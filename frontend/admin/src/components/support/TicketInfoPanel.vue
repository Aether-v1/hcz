<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useI18n } from 'vue-i18n'
import type { SupportAdminUser, SupportTicketDetail } from '@/types/support'

const props = defineProps<{
  ticket: SupportTicketDetail
  admins: SupportAdminUser[]
  acting?: boolean
}>()

const emit = defineEmits<{
  (e: 'claim'): void
  (e: 'assign', adminId: number): void
  (e: 'changePriority', priority: string): void
  (e: 'resolve', reason: string): void
  (e: 'close', reason: string): void
  (e: 'reopen', reason: string): void
}>()

const { t } = useI18n()

const selectedAdmin = ref<string>('')
const selectedPriority = ref<string>(props.ticket.priority || 'normal')

// reason dialog state
const reasonDialog = ref<'resolve' | 'close' | 'reopen' | null>(null)
const reasonText = ref('')

const openReason = (type: 'resolve' | 'close' | 'reopen') => {
  reasonText.value = ''
  reasonDialog.value = type
}

const confirmReason = () => {
  const r = reasonText.value.trim()
  if (reasonDialog.value === 'resolve') emit('resolve', r)
  else if (reasonDialog.value === 'close') emit('close', r)
  else if (reasonDialog.value === 'reopen') emit('reopen', r)
  reasonDialog.value = null
}

const doAssign = () => {
  const id = Number(selectedAdmin.value)
  if (id > 0) emit('assign', id)
}

const isResolved = computedStatus('resolved')
const isClosed = computedStatus('closed')

function computedStatus(s: string) {
  return props.ticket.status === s
}
</script>

<template>
  <div class="space-y-4">
    <!-- 分配 -->
    <div class="rounded-xl border border-border bg-card p-4">
      <h3 class="mb-2 text-sm font-semibold">{{ t('admin.support.assign') }}</h3>
      <div class="text-sm text-muted-foreground">
        {{ ticket.assigned_admin_name || (ticket.assigned_admin_id ? `#${ticket.assigned_admin_id}` : t('admin.support.unassigned')) }}
      </div>
      <div class="mt-3 flex flex-wrap items-center gap-2">
        <Button v-if="!ticket.assigned_admin_id" size="sm" :disabled="acting" @click="emit('claim')">
          {{ t('admin.support.claim') }}
        </Button>
        <Select v-model="selectedAdmin" :disabled="acting">
          <SelectTrigger class="h-8 w-[160px] text-xs">
            <SelectValue :placeholder="t('admin.support.assignToPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="a in admins" :key="a.id" :value="String(a.id)">
              {{ a.username || a.email || `#${a.id}` }}
            </SelectItem>
          </SelectContent>
        </Select>
        <Button size="sm" variant="outline" :disabled="acting || !Number(selectedAdmin)" @click="doAssign">
          {{ t('admin.support.assign') }}
        </Button>
      </div>
    </div>

    <!-- 优先级 -->
    <div class="rounded-xl border border-border bg-card p-4">
      <h3 class="mb-2 text-sm font-semibold">{{ t('admin.support.change_priority') }}</h3>
      <div class="flex items-center gap-2">
        <Select :model-value="selectedPriority" @update:model-value="(v) => { if (v) { selectedPriority = String(v); emit('changePriority', String(v)) } }" :disabled="acting">
          <SelectTrigger class="h-8 w-[140px] text-xs">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="low">{{ t('admin.support.priority_low') }}</SelectItem>
            <SelectItem value="normal">{{ t('admin.support.priority_normal') }}</SelectItem>
            <SelectItem value="high">{{ t('admin.support.priority_high') }}</SelectItem>
            <SelectItem value="urgent">{{ t('admin.support.priority_urgent') }}</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- 状态操作 -->
    <div class="rounded-xl border border-border bg-card p-4">
      <h3 class="mb-2 text-sm font-semibold">{{ t('admin.support.status') }}</h3>
      <div class="flex flex-wrap gap-2">
        <Button v-if="!isResolved && !isClosed" size="sm" variant="outline" :disabled="acting" @click="openReason('resolve')">
          {{ t('admin.support.resolve') }}
        </Button>
        <Button v-if="!isClosed" size="sm" variant="destructive" :disabled="acting" @click="openReason('close')">
          {{ t('admin.support.close') }}
        </Button>
        <Button v-if="isResolved" size="sm" variant="outline" :disabled="acting" @click="openReason('reopen')">
          {{ t('admin.support.reopen') }}
        </Button>
      </div>
    </div>

    <!-- reason dialog -->
    <Dialog :open="reasonDialog !== null" @update:open="(v) => { if (!v) reasonDialog = null }">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>
            {{ reasonDialog === 'resolve' ? t('admin.support.resolve') : reasonDialog === 'close' ? t('admin.support.close') : t('admin.support.reopen') }}
          </DialogTitle>
          <DialogDescription>
            {{ reasonDialog === 'resolve' ? t('admin.support.confirm_resolve') : reasonDialog === 'close' ? t('admin.support.confirm_close') : t('admin.support.confirm_reopen') }}
          </DialogDescription>
        </DialogHeader>
        <div class="py-2">
          <Label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.support.reason') }}</Label>
          <Textarea v-model="reasonText" rows="3" :placeholder="t('admin.support.reasonPlaceholder')" />
        </div>
        <DialogFooter>
          <Button variant="outline" @click="reasonDialog = null">{{ t('admin.common.cancel') }}</Button>
          <Button :disabled="acting" @click="confirmReason">{{ t('admin.common.confirm') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

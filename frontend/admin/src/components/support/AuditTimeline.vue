<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SupportAudit } from '@/types/support'
import { formatDate } from '@/utils/format'

const props = defineProps<{
  audits: SupportAudit[]
}>()

const { t } = useI18n()

const actionLabel = (action: string) => {
  switch (action) {
    case 'assign': return t('admin.support.action_assign')
    case 'status_change': return t('admin.support.action_status_change')
    case 'priority_change': return t('admin.support.action_priority_change')
    case 'resolve': return t('admin.support.action_resolve')
    case 'close': return t('admin.support.action_close')
    case 'reopen': return t('admin.support.action_reopen')
    case 'reply': return t('admin.support.reply')
    default: return action
  }
}

const formatVal = (v: unknown) => {
  if (v === undefined || v === null || v === '') return '-'
  if (typeof v === 'object') {
    try { return JSON.stringify(v) } catch { return String(v) }
  }
  return String(v)
}

const items = computed(() =>
  [...props.audits].sort((a, b) => (a.created_at < b.created_at ? 1 : -1)),
)
</script>

<template>
  <div class="space-y-1">
    <h3 class="mb-2 text-sm font-semibold">{{ t('admin.support.audit_log') }}</h3>
    <div v-if="items.length === 0" class="text-xs text-muted-foreground">{{ t('admin.support.no_audits') }}</div>
    <ol v-else class="relative border-l border-border pl-4 space-y-3">
      <li v-for="item in items" :key="item.id" class="relative">
        <span class="absolute -left-[21px] top-1 h-2 w-2 rounded-full bg-primary" />
        <div class="text-xs">
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium text-foreground">{{ item.admin_name || `#${item.admin_id}` }}</span>
            <span class="rounded-full border border-border bg-muted/40 px-2 py-0.5 text-[11px] text-muted-foreground">
              {{ actionLabel(item.action) }}
            </span>
            <span class="text-muted-foreground">{{ formatDate(item.created_at) }}</span>
          </div>
          <div class="mt-1 text-muted-foreground">
            <template v-if="item.before !== undefined || item.after !== undefined">
              <span class="font-mono">{{ formatVal(item.before) }}</span>
              <span class="mx-1">→</span>
              <span class="font-mono">{{ formatVal(item.after) }}</span>
            </template>
          </div>
          <div v-if="item.reason" class="mt-1 text-muted-foreground">{{ item.reason }}</div>
        </div>
      </li>
    </ol>
  </div>
</template>

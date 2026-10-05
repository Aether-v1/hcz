<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  status?: string
}>()

const { t } = useI18n()

const label = computed(() => {
  switch (props.status) {
    case 'open': return t('admin.support.status_open')
    case 'waiting_user': return t('admin.support.status_waiting_user')
    case 'waiting_support': return t('admin.support.status_waiting_support')
    case 'resolved': return t('admin.support.status_resolved')
    case 'closed': return t('admin.support.status_closed')
    default: return props.status || '-'
  }
})

const cls = computed(() => {
  switch (props.status) {
    case 'open': return 'border-info/30 bg-info/10 text-info'
    case 'waiting_user': return 'border-warning/30 bg-warning/10 text-warning'
    case 'waiting_support': return 'border-destructive/30 bg-destructive/10 text-destructive'
    case 'resolved': return 'border-success/30 bg-success/10 text-success'
    case 'closed': return 'border-border bg-muted text-muted-foreground'
    default: return 'border-border bg-muted text-muted-foreground'
  }
})
</script>

<template>
  <span class="inline-flex whitespace-nowrap rounded-full border px-2.5 py-0.5 text-xs" :class="cls">
    {{ label }}
  </span>
</template>

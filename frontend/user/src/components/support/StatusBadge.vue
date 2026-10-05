<template>
  <Badge variant="outline" size="sm" :class="toneClass">
    {{ label }}
  </Badge>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Badge } from '@/components/ui/badge'

/**
 * 工单状态 / 优先级徽标（classic / vault 共用，语义色跟随主题）。
 * status: open 蓝 / waiting_user 橙 / waiting_support 紫 / resolved 绿 / closed 灰
 * priority: low 灰 / normal 蓝 / high 橙 / urgent 红
 */
const props = defineProps<{
  kind: 'status' | 'priority'
  value: string
}>()

const { t } = useI18n()

const label = computed(() =>
  props.kind === 'status' ? t(`support.status_${props.value}`) : t(`support.priority_${props.value}`),
)

const toneClass = computed(() => {
  if (props.kind === 'status') {
    switch (props.value) {
      case 'open':
        return 'border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400'
      case 'waiting_user':
        return 'border-orange-500/30 bg-orange-500/10 text-orange-600 dark:text-orange-400'
      case 'waiting_support':
        return 'border-purple-500/30 bg-purple-500/10 text-purple-600 dark:text-purple-400'
      case 'resolved':
        return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
      case 'closed':
      default:
        return 'border-border bg-muted text-muted-foreground'
    }
  }
  switch (props.value) {
    case 'urgent':
      return 'border-red-500/30 bg-red-500/10 text-red-600 dark:text-red-400'
    case 'high':
      return 'border-orange-500/30 bg-orange-500/10 text-orange-600 dark:text-orange-400'
    case 'normal':
      return 'border-blue-500/30 bg-blue-500/10 text-blue-600 dark:text-blue-400'
    case 'low':
    default:
      return 'border-border bg-muted text-muted-foreground'
  }
})
</script>

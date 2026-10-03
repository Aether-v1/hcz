<script setup lang="ts">
// Extracted from ResellerSiteConfigPanel: a single footer-link editor row.
// Kept as its own component so v-model binds to a clean prop boundary.
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Trash2 } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'

interface FooterLink {
  name: Record<string, string>
  url: string
}

defineOptions({ name: 'FooterLinkRow' })

const props = defineProps<{
  link: FooterLink
  index: number
  activeLocale: string
}>()

const emit = defineEmits<{
  (e: 'update', value: FooterLink): void
  (e: 'remove', index: number): void
}>()

const { t } = useI18n()

const onNameInput = (v: string | number) => {
  emit('update', { ...props.link, name: { ...props.link.name, [props.activeLocale]: String(v) } })
}
const onUrlInput = (v: string | number) => {
  emit('update', { ...props.link, url: String(v) })
}
</script>

<template>
  <div
    class="mb-3 grid grid-cols-1 gap-3 rounded-xl border bg-muted/20 p-3 md:grid-cols-[1fr_1fr_auto]"
  >
    <Input
      :model-value="link.name[activeLocale]"
      @update:model-value="onNameInput"
      type="text"
      :placeholder="t('personalCenter.reseller.siteConfig.fields.linkName')"
    />
    <Input
      :model-value="link.url"
      @update:model-value="onUrlInput"
      type="text"
      placeholder="https://example.com"
    />
    <Button
      type="button"
      variant="ghost"
      size="icon"
      class="text-destructive hover:bg-destructive/10 hover:text-destructive"
      :aria-label="t('personalCenter.reseller.siteConfig.actions.remove')"
      @click="emit('remove', index)"
    >
      <Trash2 class="h-4 w-4" />
    </Button>
  </div>
</template>

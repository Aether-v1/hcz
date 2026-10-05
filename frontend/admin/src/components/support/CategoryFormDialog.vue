<script setup lang="ts">
import { reactive, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { useI18n } from 'vue-i18n'
import type { SupportCategory } from '@/types/support'

const props = defineProps<{
  open: boolean
  editing: SupportCategory | null
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'submit', payload: { code?: string; name: string; enabled: boolean; sort_order: number; default_priority: string }): void
}>()

const { t } = useI18n()

const form = reactive({
  code: '',
  name: '',
  enabled: true,
  sort_order: 0,
  default_priority: 'normal',
})

watch(
  () => props.open,
  (open) => {
    if (!open) return
    if (props.editing) {
      form.code = props.editing.code
      form.name = props.editing.name
      form.enabled = props.editing.enabled
      form.sort_order = props.editing.sort_order
      form.default_priority = props.editing.default_priority || 'normal'
    } else {
      form.code = ''
      form.name = ''
      form.enabled = true
      form.sort_order = 0
      form.default_priority = 'normal'
    }
  },
)

const submit = () => {
  if (!form.name.trim()) return
  emit('submit', {
    code: props.editing ? undefined : form.code.trim() || undefined,
    name: form.name.trim(),
    enabled: form.enabled,
    sort_order: Number(form.sort_order) || 0,
    default_priority: form.default_priority,
  })
}
</script>

<template>
  <Dialog :open="open" @update:open="(v) => emit('update:open', v)">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle>{{ editing ? t('admin.support.edit_category') : t('admin.support.add_category') }}</DialogTitle>
        <DialogDescription v-if="!editing">{{ t('admin.support.categoryCreateTip') }}</DialogDescription>
      </DialogHeader>
      <div class="space-y-4 py-2">
        <div v-if="!editing">
          <Label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.support.category_code') }}</Label>
          <Input v-model="form.code" :placeholder="t('admin.support.category_codePlaceholder')" />
        </div>
        <div>
          <Label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.support.category_name') }}</Label>
          <Input v-model="form.name" :placeholder="t('admin.support.category_namePlaceholder')" />
        </div>
        <div>
          <Label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.support.category_default_priority') }}</Label>
          <Select v-model="form.default_priority">
            <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="low">{{ t('admin.support.priority_low') }}</SelectItem>
              <SelectItem value="normal">{{ t('admin.support.priority_normal') }}</SelectItem>
              <SelectItem value="high">{{ t('admin.support.priority_high') }}</SelectItem>
              <SelectItem value="urgent">{{ t('admin.support.priority_urgent') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <Label class="mb-1 block text-xs text-muted-foreground">{{ t('admin.support.category_sort_order') }}</Label>
          <Input v-model.number="form.sort_order" type="number" />
        </div>
        <div class="flex items-center justify-between">
          <Label class="text-xs text-muted-foreground">{{ t('admin.support.category_enabled') }}</Label>
          <Switch v-model="form.enabled" />
        </div>
      </div>
      <DialogFooter>
        <Button variant="outline" @click="emit('update:open', false)">{{ t('admin.common.cancel') }}</Button>
        <Button :disabled="!form.name.trim()" @click="submit">{{ t('admin.common.confirm') }}</Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>

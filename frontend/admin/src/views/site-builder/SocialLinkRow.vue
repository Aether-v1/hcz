<script setup lang="ts">
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

defineProps<{ item: any }>()
</script>

<template>
  <div class="space-y-3 rounded-lg border border-border bg-muted/10 p-4">
    <div class="flex items-center justify-between">
      <span class="text-sm font-medium">{{ item.label }}</span>
      <Label class="flex items-center gap-2 text-sm text-muted-foreground">
        <Switch v-model="item.enabled" />
        启用
      </Label>
    </div>
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <Input v-model="item.label" placeholder="显示名称" :disabled="!item.enabled" />
      <Input v-model="item.value" :disabled="!item.enabled" :placeholder="item.key === 'email' ? 'name@example.com' : 'https://'" />
    </div>
  </div>
</template>

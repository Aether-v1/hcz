<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { HOME_ENTRY_ICONS } from '@/api/site-builder'

defineProps<{ item: any; index: number; currentLang: string }>()
const emit = defineEmits<{ remove: [index: number] }>()
</script>

<template>
  <div class="space-y-3 rounded-lg border border-border bg-muted/10 p-4">
    <div class="flex items-center justify-between">
      <span class="text-xs text-muted-foreground">导航项 {{ index + 1 }}</span>
      <div class="flex items-center gap-3">
        <Label class="flex items-center gap-2 text-xs text-muted-foreground"><Switch v-model="item.enabled" /> 启用</Label>
        <Button size="sm" variant="destructive" @click="emit('remove', index)">删除</Button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-2">
      <div class="space-y-1">
        <label class="text-xs text-muted-foreground">标题 ({{ currentLang }})</label>
        <Input v-model="item.title[currentLang]" />
      </div>
      <div class="space-y-1">
        <label class="text-xs text-muted-foreground">图标</label>
        <Select v-model="item.icon">
          <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem v-for="ic in HOME_ENTRY_ICONS" :key="ic" :value="ic">{{ ic }}</SelectItem></SelectContent>
        </Select>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
      <div class="space-y-1">
        <label class="text-xs text-muted-foreground">类型</label>
        <div class="flex gap-3 pt-2">
          <Label class="flex items-center gap-1 text-xs cursor-pointer"><input type="radio" value="internal" v-model="item.link_type" /> 内部路由</Label>
          <Label class="flex items-center gap-1 text-xs cursor-pointer"><input type="radio" value="external" v-model="item.link_type" /> 外链</Label>
        </div>
      </div>
      <div class="space-y-1 md:col-span-2">
        <label class="text-xs text-muted-foreground">{{ item.link_type === 'internal' ? '路由路径' : 'URL (http/https)' }}</label>
        <Input v-model="item.url" placeholder="/recharge 或 https://" />
      </div>
      <div class="space-y-1">
        <label class="text-xs text-muted-foreground">打开方式</label>
        <Select v-model="item.target">
          <SelectTrigger class="h-9 w-full"><SelectValue /></SelectTrigger>
          <SelectContent>
            <SelectItem value="_self">当前窗口</SelectItem>
            <SelectItem value="_blank">新窗口</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div class="space-y-1">
        <label class="text-xs text-muted-foreground">排序</label>
        <Input v-model.number="item.sort_order" type="number" />
      </div>
    </div>
  </div>
</template>

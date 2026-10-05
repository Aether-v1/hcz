<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

defineProps<{ item: any; index: number }>()
const emit = defineEmits<{ remove: [index: number] }>()
</script>

<template>
  <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/10 p-3 sm:flex-row sm:items-center">
    <div class="grid flex-1 grid-cols-1 gap-3 md:grid-cols-2">
      <Input v-model="item.name" placeholder="名称，如：帮助中心" />
      <Input v-model="item.url" placeholder="https://example.com" />
    </div>
    <Button size="sm" variant="destructive" @click="emit('remove', index)">删除</Button>
  </div>
</template>

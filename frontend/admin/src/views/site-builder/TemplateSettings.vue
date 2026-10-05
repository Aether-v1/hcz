<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { getTemplate, updateTemplate, type StorefrontTemplate } from '@/api/site-builder'
import { notifyError, notifySuccess } from '@/utils/notify'

const loading = ref(false)
const saving = ref(false)
const current = ref<StorefrontTemplate>('classic')

const options: Array<{ value: StorefrontTemplate; name: string; desc: string }> = [
  { value: 'classic', name: 'Classic', desc: '经典布局：顶部导航 + 多区块首页，信息密度高，适合综合商城。' },
  { value: 'vault', name: 'Vault', desc: 'Vault 风格：沉浸式卡片化首页，突出品牌与精选商品。' },
]

const fetchTemplate = async () => {
  loading.value = true
  try {
    const res = await getTemplate()
    const data = (res.data?.data || {}) as Record<string, any>
    const tpl = String(data.storefront_template || 'classic').trim()
    current.value = tpl === 'vault' ? 'vault' : 'classic'
  } catch {
    current.value = 'classic'
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    await updateTemplate({ storefront_template: current.value })
    notifySuccess('已保存')
  } catch {
    notifyError('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(fetchTemplate)
</script>

<template>
  <div v-if="!loading" class="space-y-6">
    <div class="rounded-xl border border-border bg-card p-6">
      <h2 class="text-lg font-semibold">前台模板</h2>
      <p class="mt-1 text-xs text-muted-foreground">当前使用：<span class="font-medium text-foreground">{{ current === 'vault' ? 'Vault' : 'Classic' }}</span></p>

      <div class="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Card
          v-for="opt in options"
          :key="opt.value"
          class="cursor-pointer transition-all"
          :class="current === opt.value ? 'border-primary ring-2 ring-primary/30' : 'hover:border-primary/40'"
          @click="current = opt.value"
        >
          <CardHeader>
            <CardTitle class="flex items-center justify-between">
              {{ opt.name }}
              <span v-if="current === opt.value" class="rounded-full bg-primary px-2 py-0.5 text-[10px] text-primary-foreground">使用中</span>
            </CardTitle>
            <CardDescription>{{ opt.desc }}</CardDescription>
          </CardHeader>
        </Card>
      </div>

      <div class="mt-5 rounded-lg border border-border bg-muted/20 p-4 text-xs text-muted-foreground leading-relaxed">
        切换模板后，User 站将立即使用新模板渲染。装修数据（首页入口 / Banner / 发现页 / 品牌）在两个模板间共享。
      </div>

      <div class="mt-5 flex justify-end">
        <Button :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存模板' }}</Button>
      </div>
    </div>
  </div>
</template>

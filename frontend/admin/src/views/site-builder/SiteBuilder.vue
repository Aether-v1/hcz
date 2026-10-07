<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { ExternalLink } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { adminAPI } from '@/api/admin'

const route = useRoute()

const tabs = [
  { key: 'brand', label: '品牌设置', to: '/site-builder/brand' },
  { key: 'home', label: '首页装修', to: '/site-builder/home' },
  { key: 'featured', label: '热门推荐', to: '/site-builder/featured' },
  { key: 'banner', label: 'Banner / 公告', to: '/site-builder/banner' },
  { key: 'discovery', label: '发现页装修', to: '/site-builder/discovery' },
  { key: 'nav-footer', label: '导航 / Footer', to: '/site-builder/nav-footer' },
]

const siteUrl = ref('')

onMounted(async () => {
  try {
    const res = await adminAPI.getPublicConfig()
    const payload = res.data?.data as any
    const url = payload?.brand?.site_url || payload?.site_url || ''
    if (typeof url === 'string' && url) siteUrl.value = url
  } catch {
    /* 静默：无预览地址时按钮不可用 */
  }
})

const openPreview = () => {
  if (!siteUrl.value) return
  window.open(siteUrl.value, '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">站点装修</h1>
        <p class="mt-1 text-xs text-muted-foreground">统一配置前台品牌、首页入口、Banner、发现页、导航</p>
      </div>
      <Button
        variant="outline"
        class="w-full sm:w-auto gap-2"
        :disabled="!siteUrl"
        :title="siteUrl ? siteUrl : '未配置前台站点地址'"
        @click="openPreview"
      >
        <ExternalLink class="h-4 w-4" />
        预览前台
      </Button>
    </div>

    <div class="flex flex-wrap gap-1 border-b border-border">
      <RouterLink
        v-for="tab in tabs"
        :key="tab.key"
        :to="tab.to"
        class="-mb-px border-b-2 px-4 py-2.5 text-sm font-medium transition-colors"
        :class="route.path.startsWith(tab.to)
          ? 'border-primary text-primary'
          : 'border-transparent text-muted-foreground hover:text-foreground'"
      >
        {{ tab.label }}
      </RouterLink>
    </div>

    <RouterView />
  </div>
</template>

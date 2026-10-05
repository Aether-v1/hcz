<template>
  <section v-if="!failed" class="db-block" :data-block-type="block.type">
    <h3 v-if="title" class="db-block__title">{{ title }}</h3>
    <component :is="component" :config="block.config" />
  </section>
</template>

<script setup lang="ts">
import { computed, onErrorCaptured, ref } from 'vue'
import type { Component } from 'vue'
import BlockBanner from './BlockBanner.vue'
import BlockCardGrid from './BlockCardGrid.vue'
import BlockBusinessRecommend from './BlockBusinessRecommend.vue'
import BlockAnnouncement from './BlockAnnouncement.vue'
import BlockExternalLink from './BlockExternalLink.vue'
import BlockCategoryEntry from './BlockCategoryEntry.vue'
import type { DiscoveryBlock } from '../../types/siteConfig'

const props = defineProps<{ block: DiscoveryBlock }>()

/** type → 组件映射；未识别的 type 返回 null（跳过并 warn） */
const TYPE_MAP: Record<string, Component> = {
  banner: BlockBanner,
  card_grid: BlockCardGrid,
  business_recommend: BlockBusinessRecommend,
  announcement: BlockAnnouncement,
  external_link: BlockExternalLink,
  category_entry: BlockCategoryEntry,
}

const failed = ref(false)

const component = computed<Component | null>(() => {
  const type = props.block.type
  const resolved = TYPE_MAP[type]
  if (!resolved) {
    console.warn(`[discovery] 未知 block type，已跳过: "${type}" (id=${props.block.id})`)
    return null
  }
  return resolved
})

const title = computed(() => props.block.title || '')

// fail-soft：单个 block 渲染抛错时，仅跳过该 block，不影响其它 block 与页面
onErrorCaptured((err, _instance, info) => {
  console.warn(`[discovery] block 渲染失败，已跳过 (type=${props.block.type}, id=${props.block.id})`, err, info)
  failed.value = true
  return false // 阻止继续向上冒泡
})
</script>

<style scoped>
.db-block {
  margin-bottom: var(--ui-section-gap, 2rem);
}
.db-block__title {
  font-size: 1rem;
  font-weight: 700;
  margin-bottom: 0.7rem;
  color: var(--ui-text-primary);
}
</style>

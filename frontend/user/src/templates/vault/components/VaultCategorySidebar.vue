<template>
  <aside class="sticky top-[88px] min-w-0">
    <div class="rounded-xl border bg-card p-2 sm:p-4">
      <div class="mb-3 flex items-center gap-2 px-1">
        <span class="h-4 w-1 flex-none rounded-full bg-primary"></span>
        <h4 class="text-sm font-bold">{{ t('products.categories') }}</h4>
      </div>
      <!-- 左侧纵向分类列表 -->
      <div class="grid max-h-[calc(100vh-7rem)] gap-1 overflow-y-auto">
        <button
          type="button"
          class="w-full rounded-lg px-2 py-2.5 text-left text-xs font-semibold transition-colors sm:px-3 sm:text-sm"
          :class="selectedCategory === null
            ? 'bg-primary/10 text-primary'
            : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
          @click="$emit('select', null)"
        >
          {{ t('products.allCategories') }}
        </button>
        <template v-for="grp in categoryGroups" :key="grp.id">
          <div class="flex min-w-0 items-center gap-0.5">
            <button
              type="button"
              class="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-2 py-2.5 text-left text-xs font-semibold transition-colors sm:px-3 sm:text-sm"
              :class="selectedCategory === grp.id
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
              @click="$emit('select', grp.id)"
            >
              <img v-if="grp.icon" :src="getImageUrl(grp.icon)" :alt="catName(grp)" loading="lazy" class="hidden h-5 w-5 flex-none rounded-md object-cover sm:block" />
              <span class="min-w-0 break-words">{{ catName(grp) }}</span>
            </button>
            <button
              v-if="grp.children.length"
              type="button"
              class="hidden h-9 w-9 flex-none place-items-center rounded-lg transition-colors hover:bg-secondary hover:text-foreground sm:grid"
              :class="expandedParentIds.includes(grp.id) ? 'text-primary' : 'text-muted-foreground'"
              :aria-expanded="expandedParentIds.includes(grp.id)"
              :aria-label="catName(grp)"
              @click="$emit('toggle', grp.id)"
            >
              <ChevronDown class="h-4 w-4 transition-transform" :class="{ 'rotate-180': expandedParentIds.includes(grp.id) }" />
            </button>
          </div>
          <template v-if="grp.children.length && expandedParentIds.includes(grp.id)">
            <button
              v-for="child in grp.children"
              :key="child.id"
              type="button"
              class="w-full min-w-0 break-words rounded-lg py-2 pl-2 pr-1 text-left text-xs font-semibold transition-colors sm:pl-9 sm:pr-3 sm:text-[13.5px]"
              :class="selectedCategory === child.id ? 'bg-primary/10 text-primary' : 'text-muted-foreground hover:bg-secondary hover:text-foreground'"
              @click="$emit('select', child.id)"
            >
              {{ catName(child) }}
            </button>
          </template>
        </template>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronDown } from 'lucide-vue-next'
import { getImageUrl } from '../../../utils/image'
import { useLocalized } from '../../../composables/useProduct'
import type { PublicCategory } from '../../../utils/category'

defineProps<{
  categoryGroups: (PublicCategory & { children: PublicCategory[] })[]
  selectedCategory: number | null
  expandedParentIds: number[]
}>()

defineEmits<{ select: [id: number | null]; toggle: [id: number] }>()

const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const catName = (cat: PublicCategory) => getLocalizedText(cat.name) || cat.slug || ''
</script>


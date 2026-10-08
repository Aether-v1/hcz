<template>
  <div class="min-h-screen bg-background">
    <ResellerConsoleTopbar :title="currentTitle" :nav-groups="navGroups" />

    <!-- 左：功能菜单栏（桌面端）｜右：内容 -->
    <div class="mx-auto flex w-full max-w-7xl gap-6 px-4 py-6 lg:px-6">
      <aside class="sticky top-20 hidden h-fit w-60 shrink-0 lg:block" :aria-label="t('resellerConsole.title')">
        <nav class="space-y-4 rounded-2xl border bg-card p-3 shadow-sm">
          <div v-for="group in navGroups" :key="group.key" class="space-y-1">
            <p class="px-3 pb-1 pt-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">{{ group.title }}</p>
            <RouterLink
              v-for="item in group.items"
              :key="item.to"
              :to="item.to"
              class="flex items-center gap-3 rounded-xl px-2.5 py-2 text-sm font-semibold transition-colors"
              :class="isActive(item.to)
                ? 'bg-primary/5 text-primary'
                : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
            >
              <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg transition-colors"
                :class="isActive(item.to) ? 'bg-primary/10 text-primary' : 'bg-accent text-muted-foreground'">
                <component :is="item.icon" :size="16" :stroke-width="1.8" />
              </span>
              <span class="truncate">{{ item.label }}</span>
            </RouterLink>
          </div>
        </nav>
      </aside>

      <!-- 主内容 -->
      <main class="min-w-0 flex-1">
      <ResellerPageState v-if="!profileReady || profileLoading" loading :title="t('resellerConsole.common.loading')" />
      <RouterView v-else-if="canRenderCurrentModule" />
      <Card v-else class="p-6 sm:p-8">
        <div class="mx-auto max-w-xl text-center">
          <span class="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <ClipboardCheck class="h-6 w-6" />
          </span>
          <h2 class="mt-4 text-lg font-bold text-foreground">{{ t('resellerConsole.dashboard.inactiveTitle') }}</h2>
          <p class="mt-2 text-sm text-muted-foreground">{{ t('resellerConsole.dashboard.inactiveDescription') }}</p>
          <Button as-child class="mt-5">
            <RouterLink to="/reseller/apply">{{ t('resellerConsole.nav.apply') }}</RouterLink>
          </Button>
        </div>
      </Card>
    </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  Banknote,
  ClipboardCheck,
  FileText,
  Globe,
  LayoutGrid,
  Settings,
  ShoppingBag,
  Tag,
  Upload,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ResellerConsoleTopbar from '../../components/reseller-console/ResellerConsoleTopbar.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import { useResellerProfile } from '../../composables/reseller/useResellerProfile'
import { canRenderResellerConsoleModule } from '../../utils/resellerConsole'

const { t } = useI18n()
const route = useRoute()
const { loading: profileLoading, state: profileState, load: loadProfile } = useResellerProfile()
const profileReady = ref(false)

type NavDef = { to: string; label: string; icon: Component }

const groupDefs: Array<{ key: string; titleKey: string; items: NavDef[] }> = [
  {
    key: 'operations',
    titleKey: 'resellerConsole.navGroups.operations',
    items: [
      { to: '/reseller', label: 'resellerConsole.nav.dashboard', icon: LayoutGrid },
      { to: '/reseller/orders', label: 'resellerConsole.nav.orders', icon: ShoppingBag },
      { to: '/reseller/finance', label: 'resellerConsole.nav.finance', icon: Banknote },
      { to: '/reseller/ledger', label: 'resellerConsole.nav.ledger', icon: FileText },
      { to: '/reseller/withdraws', label: 'resellerConsole.nav.withdraws', icon: Upload },
    ],
  },
  {
    key: 'config',
    titleKey: 'resellerConsole.navGroups.config',
    items: [
      { to: '/reseller/domains', label: 'resellerConsole.nav.domains', icon: Globe },
      { to: '/reseller/site', label: 'resellerConsole.nav.site', icon: Settings },
      { to: '/reseller/products', label: 'resellerConsole.nav.products', icon: Tag },
    ],
  },
  {
    key: 'account',
    titleKey: 'resellerConsole.navGroups.account',
    items: [{ to: '/reseller/apply', label: 'resellerConsole.nav.apply', icon: ClipboardCheck }],
  },
]

const navGroups = computed(() =>
  groupDefs.map((group) => ({
    key: group.key,
    title: t(group.titleKey),
    items: group.items.map((item) => ({ ...item, label: t(item.label) })),
  })),
)

const isActive = (path: string) => {
  if (path === '/reseller') return route.path === path
  return route.path === path || route.path.startsWith(`${path}/`)
}

const currentTitle = computed(() => {
  for (const group of navGroups.value) {
    const hit = group.items.find((item) => isActive(item.to))
    if (hit) return hit.label
  }
  return t('resellerConsole.title')
})

const canRenderCurrentModule = computed(() => canRenderResellerConsoleModule(route.path, profileState.value))

const initializeProfile = async () => {
  await loadProfile()
  profileReady.value = true
}

onMounted(initializeProfile)
</script>


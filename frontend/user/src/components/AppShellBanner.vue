<template>
  <div
    v-if="mode"
    class="pointer-events-none fixed inset-x-0 z-40"
    :class="standalone ? 'top-[calc(0.5rem+var(--safe-top))]' : 'top-[calc(57px+var(--safe-top))]'"
  >
    <div class="hcz-shell-container pointer-events-auto">
      <div class="flex items-center gap-3 rounded-2xl border bg-card px-4 py-3 text-sm shadow-md">
        <RefreshCw v-if="mode === 'update'" class="h-4 w-4 shrink-0 text-muted-foreground" />
        <WifiOff v-else-if="mode === 'offline'" class="h-4 w-4 shrink-0 text-muted-foreground" />
        <Share v-else-if="mode === 'ios'" class="h-4 w-4 shrink-0 text-muted-foreground" />
        <Smartphone v-else class="h-4 w-4 shrink-0 text-muted-foreground" />

        <p class="min-w-0 flex-1 text-foreground">{{ text }}</p>

        <button
          v-if="mode === 'update'"
          type="button"
          class="shrink-0 rounded-lg bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground"
          @click="applyUpdate"
        >
          {{ t('pwa.refresh') }}
        </button>
        <button
          v-if="mode === 'install'"
          type="button"
          class="shrink-0 rounded-lg bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground"
          @click="install"
        >
          {{ t('pwa.install') }}
        </button>
        <button
          v-if="mode === 'ios' || mode === 'install'"
          type="button"
          class="shrink-0 rounded-lg p-1.5 text-muted-foreground"
          :aria-label="t('pwa.dismiss')"
          @click="dismiss"
        >
          <X class="h-4 w-4" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RefreshCw, Share, Smartphone, WifiOff, X } from 'lucide-vue-next'
import { usePwa } from '../composables/usePwa'

/**
 * App Shell 提示条：安装 / 新版本 / 离线，一次只显示一条，优先级
 * update > offline > install > ios。安装相关提示只在触屏外壳出现，
 * 桌面端布局不受影响；关闭状态写入 localStorage，避免每次打开都提示。
 */

const INSTALL_DISMISS_KEY = 'hcz_pwa_install_hint_dismissed_at'

const { t } = useI18n()
const {
  online,
  standalone,
  canInstall,
  needRefresh,
  offlineReady,
  showIosInstallHint,
  promptInstall,
  dismissIosHint,
  updateSW,
} = usePwa()

const isTouchShell = typeof window !== 'undefined'
  && window.matchMedia?.('(pointer: coarse)').matches === true

const installDismissed = ref(
  typeof localStorage !== 'undefined' && Boolean(localStorage.getItem(INSTALL_DISMISS_KEY)),
)
const readyNoticeVisible = ref(false)

type Mode = 'update' | 'offline' | 'install' | 'ios' | 'ready'

const mode = computed<Mode | null>(() => {
  if (needRefresh.value) return 'update'
  if (!online.value) return 'offline'
  if (readyNoticeVisible.value) return 'ready'
  if (standalone.value || !isTouchShell) return null
  if (canInstall.value) return installDismissed.value ? null : 'install'
  if (showIosInstallHint.value) return 'ios'
  return null
})

const text = computed(() => {
  switch (mode.value) {
    case 'update': return t('pwa.update_available')
    case 'offline': return t('pwa.offline')
    case 'ready': return t('pwa.offline_ready')
    case 'install': return t('pwa.install_hint')
    default: return t('pwa.ios_hint')
  }
})

watch(offlineReady, (ready) => {
  if (!ready) return
  readyNoticeVisible.value = true
  window.setTimeout(() => { readyNoticeVisible.value = false }, 5000)
})

onMounted(() => {
  // 已经安装过就不再提示；appinstalled 后 standalone 检测可能滞后一拍。
  window.addEventListener('appinstalled', () => { installDismissed.value = true })
})

const install = async () => {
  const accepted = await promptInstall()
  if (accepted) standalone.value = true
  else installDismissed.value = true
}

const dismiss = () => {
  if (mode.value === 'ios') dismissIosHint()
  else {
    installDismissed.value = true
    try { localStorage.setItem(INSTALL_DISMISS_KEY, String(Date.now())) } catch { /* 隐私模式下仅本次会话隐藏 */ }
  }
}

const applyUpdate = () => {
  // 由用户主动点击才切换新版本，避免打断正在填写的表单。
  void updateSW(true)
}
</script>

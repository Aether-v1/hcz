<template>
  <div id="app" class="bg-background text-foreground" :class="{ 'hcz-storefront': !isResellerConsole }" style="min-height: 100vh; min-height: 100dvh; display: flex; flex-direction: column;">
    <Navbar v-if="showNavbar && !isResellerConsole && !isLoginOrRegister" />
    <!-- 二级/三级页面返回栏 -->
    <div v-if="showBackBar && !isResellerConsole && !isLoginOrRegister" class="fixed inset-x-0 top-0 z-50 h-14 border-b border-border bg-card/95 backdrop-blur-sm">
      <div class="hcz-shell-container flex h-full items-center">
        <button type="button" class="flex items-center gap-1 rounded-lg p-2 text-foreground transition-colors hover:bg-accent/50" @click="handleBack">
          <ChevronLeft :size="20" :stroke-width="2" />
        </button>
      </div>
    </div>
    <main class="flex-1" style="flex: 1 1 0%; display: flex; flex-direction: column;" :class="{ 'hcz-shell-main--with-bottom-nav': !isLoginOrRegister && !isResellerConsole }" @click.capture="guardPreviewFormClick" @submit.capture="guardPreviewFormSubmit">
      <div class="hcz-page-background" v-if="!isResellerConsole && !isLoginOrRegister" :class="{ 'hcz-page-background--with-top-bar': (showNavbar || showBackBar) && !isResellerConsole && !isLoginOrRegister }">
        <div class="hcz-shell-container">
          <ErrorBoundary>
            <RouterView v-slot="{ Component }">
              <component :is="Component" />
            </RouterView>
          </ErrorBoundary>
        </div>
      </div>
      <ErrorBoundary v-else>
        <RouterView v-slot="{ Component }">
          <component :is="Component" />
        </RouterView>
      </ErrorBoundary>
    </main>
    <!-- Footer 已退休：桌面端底部导航由 MobileBottomNav 响应式接管（居中浮动药囊） -->
    <BackToTop v-if="!isResellerConsole && !isLoginOrRegister" />
    <MobileBottomNav v-if="!isResellerConsole && !isLoginOrRegister" />
    <Loading :loading="appStore.loading" />
    <Toast />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChevronLeft } from 'lucide-vue-next'
import { useAppStore } from './stores/app'
import { useUserAuthStore } from './stores/userAuth'
import { useI18n } from 'vue-i18n'
import { DEV_PREVIEW_MODE } from './utils/devPreview'
import { isPrivatePreviewRoute } from './utils/devPreviewPolicy'
import { toast } from './composables/useToast'
import Navbar from './components/Navbar.vue'
import Loading from './components/Loading.vue'
import Toast from './components/Toast.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import ErrorBoundary from './components/ErrorBoundary.vue'
import BackToTop from './components/BackToTop.vue'
import MobileBottomNav from './components/MobileBottomNav.vue'

// config 由 router.beforeEach 统一加载，无需在此重复调用
const appStore = useAppStore()
const userAuthStore = useUserAuthStore()
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const isResellerConsole = computed(() => route.meta.resellerConsole === true)
const isLoginOrRegister = computed(() => route.name === 'user-login' || route.name === 'user-register' || route.name === 'user-forgot')
const showNavbar = computed(() => ['home', 'personal-center-orders', 'personal-center'].includes(String(route.name)))
const showBackBar = computed(() => !showNavbar.value && !isResellerConsole.value && !isLoginOrRegister.value)

const handleBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/')
  }
}
const privatePreviewActive = computed(() => DEV_PREVIEW_MODE && !userAuthStore.isAuthenticated && isPrivatePreviewRoute(route.name))

const guardPreviewFormClick = (event: MouseEvent) => {
  if (!privatePreviewActive.value || !(event.target instanceof Element)) return
  if (!event.target.closest('form button[type="submit"], form input[type="submit"]')) return
  event.preventDefault()
  event.stopPropagation()
  toast.info(t('devPreview.blocked'))
}

const guardPreviewFormSubmit = (event: Event) => {
  if (!privatePreviewActive.value) return
  event.preventDefault()
  event.stopPropagation()
  toast.info(t('devPreview.blocked'))
}
</script>

<style>
</style>

<template>
  <div id="app" class="bg-background text-foreground" :class="{ 'hcz-storefront': !isResellerConsole }" style="min-height: 100vh; min-height: 100dvh; display: flex; flex-direction: column;">
    <Navbar v-if="!isResellerConsole && !isLoginOrRegister" :back-only="!isTopLevelPage" />
    <main class="flex-1" style="flex: 1 1 0%; display: flex; flex-direction: column;" :class="{ 'hcz-shell-main--with-bottom-nav': isTopLevelPage && !isResellerConsole && !isLoginOrRegister }" @click.capture="guardPreviewFormClick" @submit.capture="guardPreviewFormSubmit">
      <div class="hcz-page-background" v-if="!isResellerConsole && !isLoginOrRegister" :class="{ 'hcz-page-background--with-top-bar': true }">
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
    <BackToTop v-if="!isResellerConsole && !isLoginOrRegister" />
    <MobileBottomNav v-if="isTopLevelPage && !isResellerConsole && !isLoginOrRegister" />
    <Loading :loading="appStore.loading" />
    <Toast />
    <ConfirmDialog />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
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
const isResellerConsole = computed(() => route.meta.resellerConsole === true)
const isLoginOrRegister = computed(() => route.name === 'user-login' || route.name === 'user-register' || route.name === 'user-forgot')
// 一级页面：显示完整 Navbar + 底部导航
const isTopLevelPage = computed(() => ['home', 'personal-center-orders', 'personal-center'].includes(String(route.name)))
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

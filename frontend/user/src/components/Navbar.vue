<template>
  <nav
    class="fixed top-0 left-0 right-0 z-50 border-b border-border/70 bg-card/95 backdrop-blur-xl transition-shadow"
    :class="scrolled ? 'shadow-sm' : ''"
    :style="{ transitionDuration: 'var(--ui-duration-normal)' }">
    <div class="hcz-shell-container flex h-[72px] items-center justify-between gap-4">
      <!-- Logo -->
      <router-link to="/" class="flex min-w-0 shrink-0 items-center gap-3" :title="brandSiteName">
        <img
          v-if="brandLogo"
          :src="brandLogo"
          :alt="brandSiteName"
          class="h-9 max-w-[150px] shrink-0 object-contain"
        />
        <span v-else class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary text-sm font-black text-primary-foreground">
          {{ brandInitial }}
        </span>
        <span class="max-w-[160px] truncate text-base font-bold tracking-tight text-foreground">{{ brandSiteName }}</span>
      </router-link>

      <!-- Desktop Menu -->
      <div class="hidden min-w-0 items-center gap-1 overflow-x-auto rounded-full bg-secondary/70 p-1 scrollbar-hide lg:flex" :aria-label="t('coreNav.ariaLabel')">
        <router-link v-for="item in menuItems" :key="item.key" :to="item.path"
          class="inline-flex h-8 shrink-0 items-center rounded-full px-3 text-sm text-muted-foreground whitespace-nowrap transition-colors hover:bg-card hover:text-foreground"
          :class="isCoreActive(item.key) ? 'bg-card font-semibold text-primary shadow-sm' : ''"
          :aria-current="isCoreActive(item.key) ? 'page' : undefined">
          {{ item.label }}
        </router-link>
        <Popover v-if="desktopMoreItems.length" v-model:open="desktopMoreOpen">
          <PopoverTrigger as-child>
            <Button variant="ghost" size="sm" class="h-8 gap-1 rounded-full px-3 whitespace-nowrap text-muted-foreground hover:bg-card hover:text-foreground">
              {{ t('navbar.more') }} <EllipsisVertical class="h-3.5 w-3.5" />
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-44 p-2">
            <template v-for="item in desktopMoreItems" :key="item.key">
              <router-link v-if="item.type === 'route'" :to="item.path" class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm hover:bg-accent" @click="desktopMoreOpen = false">
                <component :is="item.icon" class="h-4 w-4" />{{ item.label }}
              </router-link>
              <a v-else :href="item.path" :target="item.target" rel="noopener noreferrer" class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm hover:bg-accent" @click="desktopMoreOpen = false">
                <component :is="item.icon" class="h-4 w-4" />{{ item.label }}
              </a>
            </template>
          </PopoverContent>
        </Popover>
      </div>

      <!-- Right Side Actions -->
      <div class="flex shrink-0 items-center gap-1 lg:gap-2">
        <router-link to="/products" class="inline-flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground" :aria-label="t('nav.products')" :title="t('nav.products')">
          <Search class="h-4 w-4" />
        </router-link>
        <router-link v-if="userAuthStore.isAuthenticated" to="/notifications" class="relative inline-flex h-9 w-9 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground" :aria-label="t('notifications.title')" :title="t('notifications.title')">
          <Bell class="h-4 w-4" />
          <span v-if="notificationStore.unreadCount" class="absolute -right-1 -top-1 rounded-full bg-rose-500 px-1 text-[10px] font-bold text-white">{{ notificationStore.badgeText }}</span>
        </router-link>
        <Button v-if="!userAuthStore.isAuthenticated" as-child size="sm"
          class="hidden rounded-full bg-primary px-4 text-primary-foreground hover:bg-primary-hover lg:inline-flex">
          <router-link to="/auth/login">
            {{ t('navbar.login') }}
          </router-link>
        </Button>
        <Button v-if="userAuthStore.isAuthenticated" as-child size="sm"
          class="hidden rounded-full bg-primary px-4 text-primary-foreground hover:bg-primary-hover lg:inline-flex">
          <router-link to="/me">
            {{ t('navbar.personalCenter') }}
          </router-link>
        </Button>
        <Button v-if="userAuthStore.isAuthenticated" variant="ghost" size="icon"
          class="hidden rounded-full text-muted-foreground hover:text-destructive lg:inline-flex"
          :aria-label="t('navbar.logout')" :title="t('navbar.logout')" @click="userAuthStore.logout()">
          <LogOut class="h-4 w-4" />
        </Button>
        <!-- Theme Switcher -->
        <Button variant="ghost" size="icon" class="rounded-full text-muted-foreground" :aria-label="theme === 'dark' ? 'Light theme' : 'Dark theme'" @click="toggleTheme">
          <Sun v-if="theme === 'dark'" class="w-4 h-4" />
          <Moon v-else class="w-4 h-4" />
        </Button>

        <!-- Language Switcher (Desktop) -->
        <Popover v-model:open="langOpen">
          <PopoverTrigger as-child>
            <Button variant="ghost" size="sm" class="hidden gap-1 rounded-full text-muted-foreground xl:inline-flex">
              <Languages class="w-4 h-4" />
              <span class="text-xs font-medium uppercase tracking-wider">{{ currentLocale }}</span>
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" class="w-40 p-2">
            <div class="px-2 pb-2 mb-2 border-b">
              <span class="text-xs text-muted-foreground font-mono px-2">{{ t('navbar.selectLanguage') }}</span>
            </div>
            <button v-for="lang in languages" :key="lang.code" @click="changeLanguage(lang.code)"
              class="w-full text-left px-3 py-2.5 text-sm rounded-md transition-colors flex items-center justify-between hover:bg-accent hover:text-accent-foreground"
              :class="{ 'text-primary': appStore.locale === lang.code }">
              {{ lang.name }}
              <span v-if="appStore.locale === lang.code" class="w-1.5 h-1.5 rounded-full bg-primary"></span>
            </button>
          </PopoverContent>
        </Popover>

        <!-- Mobile Menu Button (more menu, not main nav) -->
        <Button variant="ghost" size="icon" class="lg:hidden text-muted-foreground [&_svg]:size-5"
          :aria-label="t('navbar.more')"
          @click="toggleMobileMenu">
          <EllipsisVertical />
        </Button>
      </div>
    </div>

  </nav>

  <!-- Teleport drawer outside nav to avoid backdrop-filter containing block bug -->
  <Teleport to="body">
    <!-- Mobile Drawer Overlay -->
    <Transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0">
      <div v-if="showMobileMenu" class="lg:hidden fixed inset-0 z-[60] bg-black/50" @click="showMobileMenu = false" style="overscroll-behavior: none;"></div>
    </Transition>

    <!-- Mobile Drawer (only items NOT in bottom nav) -->
    <Transition
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="translate-x-full"
      enter-to-class="translate-x-0"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="translate-x-0"
      leave-to-class="translate-x-full">
      <div v-if="showMobileMenu"
        class="lg:hidden fixed right-0 top-0 bottom-0 z-[70] w-72 max-w-[80vw] bg-card/95 backdrop-blur-xl border-l overflow-y-auto"
        style="overscroll-behavior: none;">
        <div class="p-5 space-y-1">
          <!-- Header -->
          <div class="flex items-center justify-between mb-4">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('navbar.more') }}</span>
            <Button variant="secondary" size="icon" class="[&_svg]:size-5" @click="showMobileMenu = false">
              <X />
            </Button>
          </div>

          <!-- Navigation items not in bottom nav -->
          <template v-for="item in mobileDrawerItems" :key="item.key">
            <Button v-if="item.type === 'route'" as-child variant="ghost"
              class="w-full justify-start gap-3 h-auto py-3 rounded-xl text-sm text-muted-foreground [&_svg]:size-5">
              <router-link :to="item.path" @click="showMobileMenu = false" active-class="!text-primary !bg-primary/10">
                <component :is="item.icon" class="shrink-0 opacity-60" />
                {{ item.label }}
              </router-link>
            </Button>
            <Button v-else as-child variant="ghost"
              class="w-full justify-start gap-3 h-auto py-3 rounded-xl text-sm text-muted-foreground [&_svg]:size-5">
              <a :href="item.path" :target="item.target" rel="noopener noreferrer" @click="showMobileMenu = false">
                <component :is="item.icon" class="shrink-0 opacity-60" />
                {{ item.label }}
              </a>
            </Button>
          </template>

          <template v-if="userAuthStore.isAuthenticated">
            <Button as-child variant="ghost" class="w-full justify-start gap-3 h-auto py-3 rounded-xl text-sm text-muted-foreground [&_svg]:size-5">
              <router-link to="/me/wallet" @click="showMobileMenu = false" active-class="!text-foreground !bg-secondary">
                <Wallet class="shrink-0 opacity-60" />{{ t('nav.wallet') }}
              </router-link>
            </Button>
          </template>

          <!-- Logout (login/me already in bottom nav) -->
          <Button v-if="userAuthStore.isAuthenticated" variant="ghost"
            class="w-full justify-start gap-3 h-auto py-3 rounded-xl text-sm text-destructive hover:text-destructive hover:bg-destructive/10 [&_svg]:size-5"
            @click="userAuthStore.logout(); showMobileMenu = false">
            <LogOut class="shrink-0 opacity-60" />
            {{ t('navbar.logout') }}
          </Button>

          <!-- Language Switcher -->
          <div class="mt-4 pt-4 border-t">
            <span class="text-xs text-muted-foreground font-semibold uppercase tracking-wider px-4">{{ t('navbar.selectLanguage') }}</span>
            <div class="mt-2 space-y-1">
              <button v-for="lang in languages" :key="lang.code" @click="changeLanguage(lang.code)"
                class="w-full text-left px-4 py-2.5 rounded-xl text-sm transition-colors min-h-[44px] flex items-center justify-between"
                :class="appStore.locale === lang.code
                  ? 'text-primary font-semibold'
                  : 'text-muted-foreground hover:text-foreground'">
                {{ lang.name }}
                <span v-if="appStore.locale === lang.code"
                  class="w-1.5 h-1.5 rounded-full bg-primary"></span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { useUserAuthStore } from '../stores/userAuth'
import { useNotificationStore } from '../stores/notification'
import { useTheme } from '../utils/theme'
import { getImageUrl } from '../utils/image'
import { useNavConfig } from '../composables/useNavConfig'
import { useCoreNavigation } from '../composables/useCoreNavigation'
import {
  Sun, Moon, Search, Wallet, LogOut, Languages, Bell,
  EllipsisVertical, X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

const { t } = useI18n()
const appStore = useAppStore()
const userAuthStore = useUserAuthStore()
const notificationStore = useNotificationStore()
const { theme, toggleTheme } = useTheme()
const { primaryNavItems, secondaryNavItems } = useNavConfig()
const { items: coreNavItems, isActive: isCoreActive } = useCoreNavigation()

const showMobileMenu = ref(false)
const langOpen = ref(false)
const scrolled = ref(false)
const desktopMoreOpen = ref(false)

const menuItems = coreNavItems
const desktopMoreItems = computed(() => primaryNavItems.value.filter(item => !['home', 'orders'].includes(item.key)))

// 后台配置的扩展入口保留在更多菜单，五个主入口由底栏承载。
const mobileDrawerItems = secondaryNavItems

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]

const currentLocale = computed(() => {
  const lang = languages.find(l => l.code === appStore.locale)
  if (!lang) return 'CN'
  return lang.code === 'en-US' ? 'EN' : (lang.code === 'zh-CN' ? '简' : '繁')
})


const brandSiteName = computed(() => {
  const text = String(appStore.config?.brand?.site_name || '').trim()
  return text !== '' ? text : 'HCZ'
})

const brandInitial = computed(() => brandSiteName.value.charAt(0).toUpperCase())

const brandLogo = computed(() => {
  const raw = String(appStore.config?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : '/hcz1_logo.png'
})

const toggleMobileMenu = () => {
  showMobileMenu.value = !showMobileMenu.value
}

const changeLanguage = (langCode: string) => {
  appStore.setLocale(langCode)
  langOpen.value = false
}

const handleScroll = () => {
  scrolled.value = window.scrollY > 20
}

onMounted(() => {
  window.addEventListener('scroll', handleScroll, { passive: true })
})

onUnmounted(() => {
  window.removeEventListener('scroll', handleScroll)
})
</script>

<style scoped>
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}
</style>


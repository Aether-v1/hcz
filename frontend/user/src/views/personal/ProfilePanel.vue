<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-4">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('settings.title') }}</h1>
    </div>

    <!-- 外观与语言 -->
    <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
      <!-- 主题选择 -->
      <div class="flex min-h-[52px] items-center justify-between border-b px-5 py-3">
        <div class="flex items-center gap-3">
          <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Palette :size="18" :stroke-width="1.8" />
          </div>
          <div>
            <p class="text-sm font-semibold text-foreground">{{ t('settings.appearance') }}</p>
            <p class="text-xs text-muted-foreground">{{ themeLabel }}</p>
          </div>
        </div>
        <div class="flex gap-1.5">
          <button
            v-for="opt in themeOptions"
            :key="opt.value"
            type="button"
            class="rounded-lg px-3 py-1.5 text-xs font-semibold transition-colors"
            :class="theme === opt.value ? 'bg-primary text-primary-foreground' : 'bg-accent/40 text-muted-foreground hover:bg-accent/60 hover:text-foreground'"
            @click="setTheme(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <!-- 语言选择 -->
      <div class="flex min-h-[52px] items-center justify-between px-5 py-3">
        <div class="flex items-center gap-3">
          <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Languages :size="18" :stroke-width="1.8" />
          </div>
          <div>
            <p class="text-sm font-semibold text-foreground">{{ t('settings.language') }}</p>
            <p class="text-xs text-muted-foreground">{{ localeLabel }}</p>
          </div>
        </div>
        <Select :model-value="appStore.locale" @update:model-value="(v: any) => handleLocaleChange(String(v))">
          <SelectTrigger class="h-9 w-32">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="zh-CN">简体中文</SelectItem>
            <SelectItem value="zh-TW">繁體中文</SelectItem>
            <SelectItem value="en-US">English</SelectItem>
          </SelectContent>
        </Select>
      </div>
    </div>

    <!-- 关于 -->
    <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
      <div class="flex min-h-[52px] items-center justify-between px-5 py-3">
        <div class="flex items-center gap-3">
          <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-muted-foreground">
            <Info :size="18" :stroke-width="1.8" />
          </div>
          <div>
            <p class="text-sm font-semibold text-foreground">{{ t('settings.about') }}</p>
            <p class="text-xs text-muted-foreground">{{ appStore.config?.brand?.site_name || 'HCZ' }}</p>
          </div>
        </div>
        <span class="text-xs font-medium text-muted-foreground">v{{ appVersion }}</span>
      </div>
    </div>

    <!-- 退出登录 -->
    <button
      type="button"
      class="flex min-h-[52px] w-full items-center justify-center gap-2 rounded-2xl bg-secondary px-5 py-3 text-sm font-semibold text-secondary-foreground transition-colors hover:bg-secondary/80"
      @click="handleLogout"
    >
      <LogOut :size="17" :stroke-width="1.8" />
      {{ t('navbar.logout') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Palette, Languages, Info, LogOut } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { useUserAuthStore } from '../../stores/userAuth'
import { useTheme, type Theme } from '../../utils/theme'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const { t } = useI18n()
const appStore = useAppStore()
const auth = useUserAuthStore()
const { theme, setTheme } = useTheme()
const { confirm } = useConfirmDialog()

const themeOptions: { value: Theme; label: string }[] = [
  { value: 'light', label: t('settings.themeLight') },
  { value: 'dark', label: t('settings.themeDark') },
  { value: 'system', label: t('settings.themeSystem') },
]

const themeLabel = computed(() => themeOptions.find(o => o.value === theme.value)?.label || '')

const localeMap: Record<string, string> = { 'zh-CN': '简体中文', 'zh-TW': '繁體中文', 'en-US': 'English' }
const localeLabel = computed(() => localeMap[appStore.locale] || appStore.locale)

const appVersion = computed(() => String(appStore.config?.app_version || '1.0.0'))

const handleLocaleChange = (locale: string) => {
  appStore.setLocale(locale)
}

const handleLogout = async () => {
  const approved = await confirm({
    title: t('personalCenter.myPage.logoutTitle'),
    message: t('personalCenter.myPage.logoutConfirm'),
    confirmText: t('navbar.logout'),
    variant: 'danger',
  })
  if (approved) auth.logout('/')
}
</script>

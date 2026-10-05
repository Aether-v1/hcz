<template>
  <footer class="border-t border-border bg-card text-foreground">
    <div class="hcz-shell-container pb-20 pt-7 md:pt-8 lg:pb-8">
      <div class="flex flex-col gap-6 md:flex-row md:items-center md:justify-between md:gap-10">
        <div class="min-w-0">
          <div class="flex items-center gap-2.5">
            <img v-if="brandLogo" :src="brandLogo" :alt="brandSiteName" class="h-8 max-w-[160px] object-contain" />
            <span v-else class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary text-xs font-black text-primary-foreground">
              {{ brandInitial }}
            </span>
            <span class="truncate text-sm font-bold tracking-tight">{{ brandSiteName }}</span>
          </div>
          <p v-if="brandDescription" class="mt-2 max-w-md text-xs leading-5 text-muted-foreground">{{ brandDescription }}</p>
        </div>

        <nav class="flex flex-wrap items-center gap-x-5 gap-y-3 text-sm font-medium text-muted-foreground md:justify-end" aria-label="footer">
          <router-link v-for="item in quickLinks" :key="item.path" :to="item.path"
            class="transition-colors hover:text-primary">{{ t(item.label) }}</router-link>
          <!-- 社交链接（config.social_links，外链新窗口 / email mailto:） -->
          <a
            v-for="social in socialItems"
            :key="social.key"
            :href="social.url"
            :target="social.email ? undefined : '_blank'"
            :rel="social.email ? undefined : 'noopener noreferrer'"
            class="transition-colors hover:text-primary"
          >{{ social.label }}<span v-if="!social.email"> ↗</span></a>
        </nav>
      </div>

      <div class="mt-6 flex flex-col gap-3 border-t border-border/70 pt-4 text-xs text-muted-foreground md:flex-row md:items-center md:justify-between">
        <span v-if="copyrightText">{{ copyrightText }}</span>
        <span v-else>&copy; {{ currentYear }} {{ brandSiteName }} · {{ t('footer.rights') }}</span>
        <div class="flex flex-wrap items-center gap-x-5 gap-y-2">
          <router-link to="/privacy" class="transition-colors hover:text-foreground">{{ t('footer.privacy') }}</router-link>
          <router-link to="/terms" class="transition-colors hover:text-foreground">{{ t('footer.terms') }}</router-link>
          <a v-for="link in footerLinks" :key="`${link.name}-${link.url}`" :href="link.url"
            :target="link.url.startsWith('http') ? '_blank' : undefined" rel="noopener noreferrer"
            class="transition-colors hover:text-foreground">{{ link.name }}</a>
        </div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { useSiteConfig } from '../composables/useSiteConfig'
import { getImageUrl } from '../utils/image'

const { t } = useI18n()
const appStore = useAppStore()
const { footerLinks, socialLinks, copyright } = useSiteConfig()

const brandSiteName = computed(() => {
  const siteName = appStore.config?.brand?.site_name
  return typeof siteName === 'string' && siteName.trim() ? siteName.trim() : 'HCZ'
})

const brandDescription = computed(() => {
  const desc = appStore.config?.brand?.site_description
  if (desc && typeof desc === 'object') {
    const val = desc[appStore.locale] || desc['zh-CN'] || ''
    return typeof val === 'string' ? val.trim() : ''
  }
  return ''
})

const brandInitial = computed(() => brandSiteName.value.charAt(0).toUpperCase())
const brandLogo = computed(() => {
  const raw = String(appStore.config?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : ''
})

const quickLinks = computed(() => {
  const items = [{ path: '/', label: 'nav.home' }]
  if (appStore.config?.template_mode !== 'list') items.push({ path: '/products', label: 'nav.products' })
  const builtin = appStore.config?.nav_config?.builtin || appStore.config?.navigation?.builtin
  if (!builtin || builtin.blog !== false) items.push({ path: '/blog', label: 'nav.blog' })
  if (!builtin || builtin.about !== false) items.push({ path: '/about', label: 'nav.about' })
  return items
})

const copyrightText = computed(() => copyright.value)

/** 社交链接 label 映射；未知渠道用 key 本身兜底 */
const SOCIAL_LABELS: Record<string, string> = {
  telegram: 'Telegram',
  whatsapp: 'WhatsApp',
  x: 'X',
  twitter: 'X',
  discord: 'Discord',
  email: 'Email',
}

const socialItems = computed(() => {
  const list: Array<{ key: string; label: string; url: string; email: boolean }> = []
  for (const [key, rawUrl] of Object.entries(socialLinks.value)) {
    if (!rawUrl) continue
    const isEmail = key === 'email' || rawUrl.startsWith('mailto:')
    const url = isEmail && !rawUrl.startsWith('mailto:') ? `mailto:${rawUrl}` : rawUrl
    list.push({ key, label: SOCIAL_LABELS[key] || key, url, email: isEmail })
  }
  return list
})

const currentYear = new Date().getFullYear()
</script>

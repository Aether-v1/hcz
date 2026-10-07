<template>
  <PersonalCenterOverview v-if="props.section === 'overview'" />
  <div v-else class="relative min-h-screen overflow-hidden bg-background text-foreground pt-24 pb-16">
    <div class="container relative z-10 mx-auto px-4">
      <header class="relative mb-8 overflow-hidden rounded-3xl border bg-card shadow-sm">
        <div class="relative flex flex-col gap-6 p-6 lg:flex-row lg:items-center lg:justify-between lg:p-8">
          <div class="flex min-w-0 items-center gap-4">
            <div class="flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-2xl font-black text-primary">
              {{ previewGuest ? 'D' : displayInitial }}
            </div>
            <div class="min-w-0">
              <p class="text-xs font-semibold uppercase tracking-[0.24em] text-primary">
                {{ t('personalCenter.title') }}
              </p>
              <h1 class="mt-1.5 truncate text-2xl font-black text-foreground lg:text-[2rem]">{{ previewGuest ? t('devPreview.user') : userProfileStore.displayName }}</h1>
              <p class="mt-1 truncate text-sm text-muted-foreground">{{ previewGuest ? `ID · ${t('devPreview.accountId')}` : (userProfileStore.profile?.email || t('personalCenter.subtitle')) }}</p>
            </div>
          </div>

          <div v-if="!previewGuest" class="relative flex flex-wrap items-center gap-2">
            <Badge :variant="emailVerifiedVariant" size="sm">{{ emailVerifiedLabel }}</Badge>
            <span
              v-if="userProfileStore.currentLevel"
              class="inline-flex items-center gap-1.5 rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-xs font-semibold text-primary"
            >
              <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-3.5 w-3.5 object-contain" alt="" />
              <span v-else-if="userProfileStore.currentLevel?.icon">{{ userProfileStore.currentLevel.icon }}</span>
              <Crown v-else class="h-3.5 w-3.5" />
              {{ levelName(userProfileStore.currentLevel) }}
            </span>
          </div>
        </div>
      </header>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-12">
        <aside class="lg:col-span-3">
          <div class="rounded-2xl border bg-card p-4 shadow-sm lg:sticky lg:top-24">
            <div class="hidden flex-col gap-0.5 lg:flex">
              <template v-for="item in visibleSectionItems" :key="item.key">
              <p v-if="item.key === 'overview' || item.key === 'giftCard' || item.key === 'affiliate'"
                class="px-4 pt-4 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
                {{ t(`personalCenter.groups.${item.key === 'giftCard' ? 'orders' : item.key}`) }}
              </p>
              <button
                type="button"
                @click="switchSection(item.key)"
                class="group relative flex w-full items-center gap-2.5 rounded-lg py-2.5 pl-4 pr-3 text-left text-sm font-semibold transition-colors"
                :class="currentSection === item.key
                  ? 'bg-primary/10 text-primary'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
              >
                <span
                  class="absolute left-0 top-1/2 h-5 w-1 -translate-y-1/2 rounded-r-full transition-all"
                  :class="currentSection === item.key ? 'bg-primary' : 'bg-transparent'"
                ></span>
                <component :is="item.icon" class="h-4 w-4 shrink-0" />
                <span class="truncate">{{ t(item.label) }}</span>
              </button>
              </template>
            </div>

            <div class="lg:hidden">
              <div class="flex gap-1.5 overflow-x-auto pb-1">
                <button
                  v-for="item in visibleSectionItems"
                  :key="item.key"
                  type="button"
                  @click="switchSection(item.key)"
                  class="shrink-0 rounded-lg px-3.5 py-2 text-xs font-semibold transition-colors"
                  :class="currentSection === item.key
                    ? 'bg-primary/10 text-primary'
                    : 'text-muted-foreground hover:bg-accent hover:text-foreground'"
                >
                  <span class="flex items-center gap-1.5">
                    <component :is="item.icon" class="h-3.5 w-3.5" />
                    <span>{{ t(item.label) }}</span>
                  </span>
                </button>
              </div>
            </div>
          </div>
        </aside>

        <section class="space-y-6 lg:col-span-9">
          <Alert
            v-if="globalAlert"
            :variant="pageAlertVariant(globalAlert.level)"
            :class="pageAlertToneClass(globalAlert.level)"
          >
            <AlertDescription>{{ globalAlert.message }}</AlertDescription>
          </Alert>

          <template v-if="currentSection === 'overview'">
            <PersonalOverviewShortcuts />
            <PersonalNotificationsEntry />
            <PersonalUsdtEntry />
            <!-- 数据一览 -->
            <div class="grid gap-4 sm:grid-cols-2">
              <StatCard :label="t('personalCenter.memberLevel.currentLevel')" :icon="Crown" tone="accent">
                <template #value>
                  <span class="flex items-center gap-1.5">
                    <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" class="h-5 w-5 shrink-0 object-contain" alt="" />
                    <span class="truncate">{{ levelName(userProfileStore.currentLevel) }}</span>
                  </span>
                </template>
              </StatCard>
              <StatCard
                :label="t('personalCenter.overview.accountLabel')"
                :icon="ShieldCheck"
                :tone="emailVerifiedVariant === 'success' ? 'success' : 'warning'"
              >
                <template #value>
                  <Badge :variant="emailVerifiedVariant" size="sm">{{ emailVerifiedLabel }}</Badge>
                </template>
              </StatCard>
            </div>

          </template>

          <ProfilePanel v-else-if="currentSection === 'profile'" />
          <SecurityPanel v-else-if="currentSection === 'security'" />
          <AffiliatePanel v-else-if="currentSection === 'affiliate'" />
          <InvitationPanel v-else-if="currentSection === 'invitation'" />
          <div v-else-if="currentSection === 'reseller' && canAccessResellerConsole" class="rounded-2xl border bg-card p-6 shadow-sm">
            <h2 class="text-xl font-bold text-foreground">{{ t('resellerConsole.title') }}</h2>
            <p class="mt-2 text-sm text-muted-foreground">{{ t('resellerConsole.dashboard.description') }}</p>
            <Button as-child class="mt-5">
              <router-link to="/reseller">{{ t('resellerConsole.nav.dashboard') }}</router-link>
            </Button>
          </div>
          <GiftCardPanel v-else-if="currentSection === 'giftCard'" />
          <ApiPanel v-else-if="currentSection === 'api'" />
          <ProfilePanel v-else />
        </section>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUserAuthStore } from '../stores/userAuth'
import { isGuestDevPreview } from '../utils/devPreview'
import PersonalCenterOverview from './personal/PersonalCenterOverview.vue'
import PersonalNotificationsEntry from '../components/PersonalNotificationsEntry.vue'
import PersonalOverviewShortcuts from '../components/PersonalOverviewShortcuts.vue'
import PersonalUsdtEntry from '../components/PersonalUsdtEntry.vue'
import { Crown, ShieldCheck } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { pageAlertVariant, pageAlertToneClass } from '../utils/alerts'
import StatCard from '../components/shared/StatCard.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import ProfilePanel from './personal/ProfilePanel.vue'
import SecurityPanel from './personal/SecurityPanel.vue'
import GiftCardPanel from './personal/GiftCardPanel.vue'
import AffiliatePanel from './personal/AffiliatePanel.vue'
import InvitationPanel from './personal/InvitationPanel.vue'
import ApiPanel from './personal/ApiPanel.vue'
import { usePersonalCenter, type PersonalSection } from '../composables/usePersonalCenter'

const { t } = useI18n()
const auth = useUserAuthStore()
const previewGuest = computed(() => isGuestDevPreview(auth.isAuthenticated))

const props = withDefaults(defineProps<{ section?: PersonalSection }>(), {
  section: 'overview',
})

const {
  userProfileStore, canAccessResellerConsole, visibleSectionItems, currentSection, globalAlert,
  displayInitial, switchSection,
  emailVerifiedLabel, emailVerifiedVariant, isImagePath, levelName,
} = usePersonalCenter(() => props.section)
</script>


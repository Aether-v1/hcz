<template>
  <div class="text-foreground">
    <div>
      <!-- 未登录欢迎 -->
      <section v-if="!auth.isAuthenticated && !previewGuest" class="mt-6 overflow-hidden rounded-3xl border p-7 sm:p-10">
        <div class="mb-6 grid h-14 w-14 place-items-center rounded-2xl text-xl font-black text-primary-foreground" style="background: var(--ui-accent)">HCZ</div>
        <h2 class="max-w-md text-2xl font-bold sm:text-3xl">{{ t('personalCenter.myPage.welcome') }}</h2>
        <p class="mt-3 max-w-md text-sm leading-6 text-muted-foreground">{{ t('personalCenter.myPage.guestHint') }}</p>
        <div class="mt-7 flex flex-wrap gap-3">
          <Button as-child><RouterLink to="/auth/login">{{ t('navbar.login') }}</RouterLink></Button>
          <Button as-child variant="outline"><RouterLink to="/auth/register">{{ t('personalCenter.myPage.register') }}</RouterLink></Button>
        </div>
      </section>

      <!-- 已登录 -->
      <template v-else>
        <!-- 顶部用户信息：融合页面背景 -->
        <section class="flex items-start gap-3 px-1 py-1">
          <!-- 头像 -->
          <RouterLink to="/me/profile" class="shrink-0 transition-opacity hover:opacity-80">
            <div v-if="previewGuest" class="grid h-14 w-14 place-items-center rounded-full bg-accent-soft text-accent">
              <UserRound :size="26" />
            </div>
            <div v-else-if="!profileReady || profile.loadingProfile" class="h-14 w-14 animate-pulse rounded-full bg-accent-soft"></div>
            <div v-else-if="profile.profile" class="grid h-14 w-14 place-items-center rounded-full bg-accent-soft text-xl font-bold text-accent">
              {{ profileInitial }}
            </div>
            <div v-else class="grid h-14 w-14 place-items-center rounded-full bg-accent-soft text-accent">
              <UserRound :size="26" />
            </div>
          </RouterLink>

          <!-- 用户信息 -->
          <div class="min-w-0 flex-1 pt-1">
            <h2 class="truncate text-lg font-bold text-foreground sm:text-xl">
              {{ previewGuest ? t('devPreview.user') : profile.displayName }}
            </h2>
            <p class="mt-0.5 truncate text-xs text-muted-foreground">
              {{ previewGuest ? `ID · ${t('devPreview.accountId')}` : (profile.profile?.email || profile.profile?.id ? `ID · ${profile.profile?.id}` : t('personalCenter.subtitle')) }}
            </p>
          </div>
        </section>

        <!-- 资产卡片：USDT 背景图 -->
        <RouterLink to="/me/wallet" class="wallet-card relative mt-4 block overflow-hidden rounded-2xl p-3.5 transition-transform hover:scale-[1.01]">
          <div class="relative flex items-start justify-between">
            <div class="min-w-0">
              <span class="wallet-card__label">{{ t('personalCenter.myPage.available') }}</span>
              <p v-if="previewGuest" class="wallet-card__amount">-- USDT</p>
              <div v-else-if="walletLoading" class="wallet-card__skeleton"></div>
              <p v-else class="wallet-card__amount">{{ walletAmount }}</p>
              <p v-if="walletError && !walletLoading && !previewGuest" class="wallet-card__hint">{{ t('personalCenter.myPage.walletFailed') }}</p>
              <p v-else-if="frozenAmount && !walletLoading && !previewGuest" class="wallet-card__hint">{{ t('personalCenter.myPage.frozen') }} {{ frozenAmount }}</p>
            </div>
          </div>
        </RouterLink>

        <!-- 快捷入口：一行四列 -->
        <section class="mt-4">
          <div class="grid grid-cols-4 gap-1 rounded-2xl border bg-card p-2.5 shadow-sm">
            <RouterLink to="/c2c" class="flex flex-col items-center gap-1.5 rounded-xl px-2 py-2.5 transition-colors hover:bg-accent/40">
              <div class="grid h-10 w-10 place-items-center rounded-[14px] bg-info-soft text-info">
                <ArrowLeftRight :size="21" :stroke-width="2" />
              </div>
              <span class="text-xs font-medium text-foreground">C2C</span>
            </RouterLink>
            <RouterLink to="/me/invitation" class="flex flex-col items-center gap-1.5 rounded-xl px-2 py-2.5 transition-colors hover:bg-accent/40">
              <div class="grid h-10 w-10 place-items-center rounded-[14px] bg-success-soft text-success">
                <UserRoundPlus :size="21" :stroke-width="2" />
              </div>
              <span class="text-xs font-medium text-foreground">邀请中心</span>
            </RouterLink>
            <RouterLink to="/me/reseller" class="flex flex-col items-center gap-1.5 rounded-xl px-2 py-2.5 transition-colors hover:bg-accent/40">
              <div class="grid h-10 w-10 place-items-center rounded-[14px] bg-violet-500/10 text-violet-700 dark:bg-violet-400/20 dark:text-violet-300">
                <Store :size="21" :stroke-width="2" />
              </div>
              <span class="text-xs font-medium text-foreground">分销中心</span>
            </RouterLink>
            <RouterLink to="/me/points" class="flex flex-col items-center gap-1.5 rounded-xl px-2 py-2.5 transition-colors hover:bg-accent/40">
              <div class="grid h-10 w-10 place-items-center rounded-[14px] bg-warm-soft text-warm">
                <Coins :size="21" :stroke-width="2" />
              </div>
              <span class="text-xs font-medium text-foreground">积分</span>
            </RouterLink>
          </div>
        </section>

        <!-- 更多功能：礼品卡兑换 / API 对接 -->
        <section class="mt-4">
          <div class="overflow-hidden rounded-2xl border bg-card shadow-sm">
            <RouterLink to="/me/gift-cards" class="menu-item flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40">
              <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-warm-soft text-warm">
                <Gift :size="19" :stroke-width="2" />
              </div>
              <span class="flex-1 text-sm font-medium text-foreground">礼品卡兑换</span>
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
            </RouterLink>
            <div class="h-px bg-border/60"></div>
            <RouterLink to="/me/api" class="menu-item flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40">
              <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-info-soft text-info">
                <Code2 :size="19" :stroke-width="2" />
              </div>
              <span class="flex-1 text-sm font-medium text-foreground">API 对接</span>
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
            </RouterLink>
          </div>
        </section>

        <!-- 菜单卡：iOS Grouped List -->
        <section class="mt-4">
          <div class="menu-group overflow-hidden rounded-2xl border border-border/60 bg-card">
            <RouterLink to="/me/security" class="menu-item flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40">
              <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-success-soft text-success">
                <LockKeyhole :size="19" :stroke-width="2" />
              </div>
              <span class="min-w-0 flex-1 text-sm font-medium text-foreground">{{ t('personalCenter.tabs.security') }}</span>
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
            </RouterLink>
            <RouterLink to="/support" class="menu-item flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40">
              <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-primary-soft text-primary">
                <CircleHelp :size="19" :stroke-width="2" />
              </div>
              <span class="min-w-0 flex-1 text-sm font-medium text-foreground">{{ t('support.help_center') }}</span>
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
            </RouterLink>
            <RouterLink to="/support/tickets" class="menu-item flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40">
              <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-violet-500/10 text-violet-700 dark:bg-violet-400/20 dark:text-violet-300">
                <Ticket :size="19" :stroke-width="2" />
              </div>
              <span class="min-w-0 flex-1 text-sm font-medium text-foreground">{{ t('support.tickets_entry') }}</span>
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
            </RouterLink>
            <RouterLink to="/me/settings" class="menu-item flex min-h-[52px] items-center gap-3 px-5 py-3 transition-colors hover:bg-accent/40">
              <div class="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-surface-soft text-ink-secondary">
                <Settings :size="19" :stroke-width="2" />
              </div>
              <span class="min-w-0 flex-1 text-sm font-medium text-foreground">{{ t('personalCenter.tabs.settings') }}</span>
              <ChevronRight :size="16" class="shrink-0 text-muted-foreground/50" />
            </RouterLink>
          </div>
        </section>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRight, CircleHelp, Coins, Gift, Code2, LockKeyhole, UserRoundPlus, Settings, Ticket, UserRound, ArrowLeftRight, Store } from 'lucide-vue-next'
import { walletAPI } from '../../api'
import type { WalletAccountData } from '../../api/types'
import { Button } from '@/components/ui/button'
import { useUserAuthStore } from '../../stores/userAuth'
import { useUserProfileStore } from '../../stores/userProfile'
import { formatUsdt } from '../../utils/money'
import { isGuestDevPreview } from '../../utils/devPreview'

const { t } = useI18n()
const auth = useUserAuthStore()
const profile = useUserProfileStore()
const wallet = ref<WalletAccountData | null>(null)
const profileReady = ref(false)
const walletLoading = ref(false)
const walletError = ref(false)
const previewGuest = computed(() => isGuestDevPreview(auth.isAuthenticated))

const profileInitial = computed(() => profile.displayName.trim().slice(0, 1).toUpperCase() || 'U')
const walletAmount = computed(() => {
  if (walletError.value || !wallet.value) return '-- USDT'
  const amount = formatUsdt(wallet.value.available_balance, 'USDT')
  return amount === '--' ? '-- USDT' : amount
})
const frozenAmount = computed(() => {
  if (!wallet.value || walletError.value) return ''
  const amount = formatUsdt(wallet.value.frozen_balance, 'USDT')
  return amount === '--' ? '' : amount
})

const loadProfile = async () => {
  profileReady.value = false
  await profile.loadProfile()
  profileReady.value = true
}
const loadWallet = async () => {
  walletLoading.value = true
  walletError.value = false
  wallet.value = null
  try {
    const response = await walletAPI.account()
    const account = response.data?.data as WalletAccountData | undefined
    if (!account || formatUsdt(account.available_balance) === '--' || account.currency !== 'USDT') {
      throw new Error('Invalid wallet response')
    }
    wallet.value = account
  } catch {
    walletError.value = true
  } finally {
    walletLoading.value = false
  }
}

const loadPrivateData = () => {
  void loadProfile()
  void loadWallet()
}

onMounted(() => {
  if (auth.isAuthenticated) loadPrivateData()
})
watch(() => auth.isAuthenticated, (loggedIn) => {
  if (loggedIn) loadPrivateData()
  else {
    profileReady.value = false
    wallet.value = null
    walletError.value = false
  }
})
</script>

<style scoped>
/* iOS Grouped List：分割线从图标右侧开始，不顶到最左 */
.menu-item {
  position: relative;
}
.menu-item:not(:last-child)::after {
  content: '';
  position: absolute;
  left: 3.25rem; /* px-5(20px) + icon(18px) + gap(12px) = 50px */
  right: 1.25rem;
  bottom: 0;
  height: 1px;
  background: var(--border);
  opacity: 0.35;
}

.wallet-card {
  background: var(--wallet-card-bg) center/cover no-repeat;
  color: var(--wallet-card-fg);
}
.wallet-card__label {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--wallet-card-fg-muted);
}
.wallet-card__amount {
  margin-top: 0.375rem;
  font-size: 1.5rem;
  font-weight: 700;
  letter-spacing: -0.025em;
  color: var(--wallet-card-fg);
}
.wallet-card__skeleton {
  margin-top: 0.375rem;
  height: 2rem;
  width: 8rem;
  border-radius: 0.5rem;
  background: color-mix(in oklab, var(--wallet-card-fg) 12%, transparent);
  animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
.wallet-card__hint {
  margin-top: 0.25rem;
  font-size: 0.75rem;
  color: color-mix(in oklab, var(--wallet-card-fg) 55%, transparent);
}
.wallet-card__action {
  display: inline-flex;
  align-items: center;
  gap: 0.125rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: color-mix(in oklab, var(--wallet-card-fg) 80%, transparent);
}
</style>

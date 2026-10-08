<template>
  <div class="user-summary-card" :class="{ 'user-summary-card--loading': walletLoading }">
    <div class="user-summary-card__user">
      <!-- Avatar + Name -->
      <div class="user-summary-card__profile">
        <div class="user-summary-card__avatar">
          <img v-if="userAvatar" :src="userAvatar" :alt="displayName" class="user-summary-card__avatar-img" />
          <component v-else :is="UserRound" :size="22" :stroke-width="1.6" class="user-summary-card__avatar-icon" aria-hidden="true" />
        </div>
        <div class="user-summary-card__info">
          <span class="user-summary-card__name">{{ displayName }}</span>
          <span class="user-summary-card__uid" v-if="isAuthenticated && user?.username">@{{ user.username }}</span>
        </div>
      </div>

      <!-- Available Balance + Points -->
      <div class="user-summary-card__balance">
        <div class="user-summary-card__balance-col">
          <span class="user-summary-card__balance-label">{{ t('homeV2.userSummary.availableBalance') }}</span>
          <div class="user-summary-card__balance-value">
            <template v-if="walletLoading">
              <span class="user-summary-card__skeleton user-summary-card__skeleton--balance" />
            </template>
            <template v-else-if="walletError || !isAuthenticated">
              <span class="user-summary-card__balance-error">-- USDT</span>
            </template>
            <template v-else>
              <span class="user-summary-card__balance-amount">{{ formattedBalance }}</span>
              <span class="user-summary-card__balance-currency">USDT</span>
            </template>
          </div>
        </div>
        <div class="user-summary-card__balance-col user-summary-card__points">
          <span class="user-summary-card__balance-label">{{ t('homeV2.userSummary.points') }}</span>
          <div class="user-summary-card__balance-value">
            <template v-if="pointsLoading">
              <span class="user-summary-card__skeleton user-summary-card__skeleton--balance" />
            </template>
            <template v-else-if="pointsError || !isAuthenticated">
              <span class="user-summary-card__balance-error">--</span>
            </template>
            <template v-else>
              <span class="user-summary-card__balance-amount">{{ formattedPoints }}</span>
            </template>
          </div>
        </div>
      </div>

      <!-- Invite CTA -->
      <RouterLink to="/me/invitation" class="user-summary-card__cta">
        {{ t('homeV2.userSummary.invite') }}
      </RouterLink>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { UserRound } from 'lucide-vue-next'
import { useUserAuthStore } from '../../stores/userAuth'
import { walletAPI, pointsAPI } from '../../api'

const { t } = useI18n()
const auth = useUserAuthStore()

const isAuthenticated = computed(() => auth.isAuthenticated)
const user = computed(() => auth.user)

const displayName = computed(() => {
  if (!isAuthenticated.value) return t('homeV2.userSummary.notLoggedIn')
  const u = user.value
  return u?.nickname || u?.name || u?.username || t('homeV2.userSummary.member')
})

const userAvatar = computed(() => user.value?.avatar || user.value?.avatar_url || '')

// ===== Wallet =====
const walletLoading = ref(false)
const walletError = ref(false)
const availableBalance = ref('')

const formattedBalance = computed(() => {
  const raw = availableBalance.value
  if (!raw) return '0.00'
  const num = Number(raw)
  if (Number.isNaN(num)) return raw
  return num.toFixed(2)
})

const fetchWallet = async () => {
  if (!isAuthenticated.value) return
  walletLoading.value = true
  walletError.value = false
  try {
    const res = await walletAPI.account()
    availableBalance.value = String(res.data?.data?.available_balance ?? '0')
  } catch {
    walletError.value = true
  } finally {
    walletLoading.value = false
  }
}

// ===== Points =====
const pointsLoading = ref(false)
const pointsError = ref(false)
const pointsBalance = ref('')

const formattedPoints = computed(() => {
  const raw = pointsBalance.value
  if (!raw) return '0'
  const num = Number(raw)
  if (Number.isNaN(num)) return raw
  return num.toLocaleString()
})

const fetchPoints = async () => {
  if (!isAuthenticated.value) return
  pointsLoading.value = true
  pointsError.value = false
  try {
    const res = await pointsAPI.account()
    pointsBalance.value = String(res.data?.data?.balance ?? '0')
  } catch {
    pointsError.value = true
  } finally {
    pointsLoading.value = false
  }
}

onMounted(() => {
  if (isAuthenticated.value) {
    fetchWallet()
    fetchPoints()
  }
})

watch(isAuthenticated, (val) => {
  if (val) {
    fetchWallet()
    fetchPoints()
  } else {
    availableBalance.value = ''
    walletError.value = false
    pointsBalance.value = ''
    pointsError.value = false
  }
})
</script>

<style scoped>
.user-summary-card {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 280px;
  padding: 24px;
  border-radius: var(--radius-xl, 20px);
  background: var(--color-surface-elevated, #1a1a22);
  border: 1px solid var(--color-hairline, rgba(255, 255, 255, 0.06));
  box-sizing: border-box;
}

/* ===== Guest ===== */
/* ===== User Profile ===== */
.user-summary-card__user {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 20px;
}

.user-summary-card__profile {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-summary-card__avatar {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  overflow: hidden;
  background: var(--brand-primary, #4f46e5);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.user-summary-card__avatar-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.user-summary-card__avatar-icon {
  color: #fff;
}

.user-summary-card__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.user-summary-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-ink-primary, #fff);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-summary-card__uid {
  font-size: 12px;
  color: var(--color-ink-muted, rgba(255, 255, 255, 0.4));
}

.user-summary-card__uid--muted {
  font-style: italic;
  opacity: 0.7;
}

.user-summary-card__balance {
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 14px 16px;
  border-radius: var(--radius-md, 10px);
  background: var(--color-surface-soft, rgba(255, 255, 255, 0.04));
}

.user-summary-card__balance-col {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.user-summary-card__points {
  text-align: right;
  align-items: flex-end;
}

.user-summary-card__points .user-summary-card__balance-value {
  justify-content: flex-end;
}

.user-summary-card__balance-label {
  font-size: 11px;
  color: var(--color-ink-secondary, rgba(255, 255, 255, 0.5));
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.user-summary-card__balance-value {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.user-summary-card__balance-amount {
  font-size: 18px;
  font-weight: 700;
  color: var(--color-ink-primary, #fff);
  font-variant-numeric: tabular-nums;
}

.user-summary-card__balance-currency {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-ink-muted, rgba(255, 255, 255, 0.4));
}

.user-summary-card__balance-error {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-ink-muted, rgba(255, 255, 255, 0.35));
}

.user-summary-card__cta {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: auto;
  padding: 13px 16px;
  border-radius: var(--radius-md, 12px);
  background: linear-gradient(135deg, var(--brand-primary, #4f46e5), var(--brand-primary-dark, #7c3aed));
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.2px;
  text-decoration: none;
  box-shadow: 0 8px 20px rgba(79, 70, 229, 0.3);
  transition: transform 0.15s ease, box-shadow 0.15s ease, filter 0.15s ease;
}

.user-summary-card__cta:hover {
  transform: translateY(-1px);
  filter: brightness(1.06);
  box-shadow: 0 10px 24px rgba(79, 70, 229, 0.38);
}

.user-summary-card__cta:active {
  transform: translateY(0);
  filter: brightness(0.98);
}

/* ===== Skeleton ===== */
.user-summary-card__skeleton {
  display: inline-block;
  border-radius: 6px;
  background: linear-gradient(
    90deg,
    var(--color-surface-muted, rgba(255, 255, 255, 0.06)) 25%,
    var(--color-surface-soft, rgba(255, 255, 255, 0.1)) 50%,
    var(--color-surface-muted, rgba(255, 255, 255, 0.06)) 75%
  );
  background-size: 200% 100%;
  animation: user-summary-shimmer 1.5s infinite;
}

.user-summary-card__skeleton--balance {
  width: 72px;
  height: 20px;
}

@keyframes user-summary-shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
</style>

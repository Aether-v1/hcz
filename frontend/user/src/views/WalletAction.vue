<template>
  <div class="bg-background pb-12 pt-24 text-foreground">
    <div class="container mx-auto max-w-6xl px-4">
      <router-link to="/me/wallet" class="mb-5 inline-flex text-sm text-muted-foreground hover:text-foreground">
        ← {{ t('nav.wallet') }}
      </router-link>
      <WalletWithdrawalHistory v-if="isHistory" />
      <WalletWithdrawal v-else />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WalletWithdrawal from './personal/WalletWithdrawal.vue'
import WalletWithdrawalHistory from './personal/WalletWithdrawalHistory.vue'

const route = useRoute()
const { t } = useI18n()
const isHistory = computed(() => route.name === 'wallet-withdrawal-history')
</script>

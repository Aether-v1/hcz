<template>
  <div class="bg-background pb-12 text-foreground" :class="isVault ? 'pt-6' : 'pt-24'">
    <div class="container mx-auto max-w-6xl px-4">
      <WalletPanel />
    </div>
  </div>
</template>

<script setup lang="ts">
import WalletPanel from './personal/WalletPanel.vue'
import { getActiveTemplate } from '../templates/registry'

const isVault = getActiveTemplate() === 'vault'
</script>

<template>
  <div class="bg-background pb-12 text-foreground" :class="isVault ? 'pt-6' : 'pt-24'">
    <div class="container mx-auto max-w-6xl px-4">
      <OrdersPanel />
    </div>
  </div>
</template>

<script setup lang="ts">
import OrdersPanel from './personal/OrdersPanel.vue'
import { getActiveTemplate } from '../templates/registry'

const isVault = getActiveTemplate() === 'vault'
</script>

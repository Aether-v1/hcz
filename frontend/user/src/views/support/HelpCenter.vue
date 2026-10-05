<template>
  <div class="min-h-screen bg-background pb-16 pt-20 text-foreground">
    <div class="container mx-auto max-w-3xl px-4">
      <!-- Header -->
      <div class="mt-8">
        <h1 class="text-2xl font-bold tracking-tight md:text-3xl">{{ t('support.help_center') }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ t('support.help_subtitle') }}</p>
      </div>

      <!-- Entry cards -->
      <div class="mt-8 grid gap-4 sm:grid-cols-2">
        <router-link
          to="/support/tickets"
          class="rounded-2xl border bg-card p-6 shadow-sm transition-colors hover:border-primary/50"
        >
          <MessageSquare class="h-8 w-8 text-primary" />
          <h2 class="mt-3 font-semibold">{{ t('support.my_tickets') }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('support.my_tickets_hint') }}</p>
        </router-link>
        <router-link
          to="/support/tickets/new"
          class="rounded-2xl border bg-card p-6 shadow-sm transition-colors hover:border-primary/50"
        >
          <CirclePlus class="h-8 w-8 text-primary" />
          <h2 class="mt-3 font-semibold">{{ t('support.create_ticket') }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('support.create_ticket_hint') }}</p>
        </router-link>
      </div>

      <!-- 充值/退款引导：优先走订单售后 -->
      <div class="mt-6 flex flex-wrap items-center gap-2 rounded-xl border border-warning/30 bg-warning/5 p-4 text-sm text-foreground">
        <AlertTriangle class="h-4 w-4 flex-none text-warning" />
        <span>{{ t('support.recharge_aftersale_warning') }}</span>
        <router-link to="/me/orders" class="font-medium text-primary hover:underline">
          {{ t('support.go_aftersale') }}
        </router-link>
      </div>

      <!-- FAQ 占位 -->
      <div class="mt-8 rounded-2xl border bg-card p-6 shadow-sm">
        <h2 class="font-semibold">{{ t('support.faq') }}</h2>
        <div class="mt-3 divide-y divide-border text-sm">
          <details class="py-3">
            <summary class="cursor-pointer font-medium">{{ t('support.faq_q1') }}</summary>
            <p class="mt-2 text-muted-foreground">{{ t('support.faq_a1') }}</p>
          </details>
          <details class="py-3">
            <summary class="cursor-pointer font-medium">{{ t('support.faq_q2') }}</summary>
            <p class="mt-2 text-muted-foreground">{{ t('support.faq_a2') }}</p>
          </details>
          <details class="py-3">
            <summary class="cursor-pointer font-medium">{{ t('support.faq_q3') }}</summary>
            <p class="mt-2 text-muted-foreground">{{ t('support.faq_a3') }}</p>
          </details>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertTriangle, CirclePlus, MessageSquare } from 'lucide-vue-next'

const { t } = useI18n()
</script>

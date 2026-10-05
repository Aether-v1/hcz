<template>
  <nav class="hcz-bottom-nav theme-safe-bottom" :aria-label="t('coreNav.ariaLabel')">
    <div class="hcz-bottom-nav__inner">
      <router-link
        v-for="item in items"
        :key="item.key"
        :to="item.path"
        class="hcz-bottom-nav__item"
        :class="{ 'is-active': isActive(item.key) }"
        :aria-current="isActive(item.key) ? 'page' : undefined"
      >
        <span class="hcz-bottom-nav__icon">
          <component :is="item.icon" :size="21" :stroke-width="isActive(item.key) ? 2.3 : 1.9" aria-hidden="true" />
          <span v-if="item.key === 'me' && notificationStore.unreadCount > 0" class="hcz-bottom-nav__dot" />
        </span>
        <span>{{ item.label }}</span>
      </router-link>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useCoreNavigation } from '../composables/useCoreNavigation'
import { useNotificationStore } from '../stores/notification'

const { t } = useI18n()
const { items, isActive } = useCoreNavigation()
const notificationStore = useNotificationStore()
</script>

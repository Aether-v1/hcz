<template>
  <div class="space-y-6 invitation-panel-enter">
    <div class="rounded-2xl border bg-card p-7 shadow-sm">
      <PanelHeading
        :title="t('personalCenter.invitation.title')"
        :description="t('personalCenter.invitation.subtitle')"
        :icon="Share2"
      >
        <template #actions>
          <Badge variant="accent" size="sm">{{ t('personalCenter.tabs.invitation') }}</Badge>
        </template>
      </PanelHeading>

      <!-- 加载中 -->
      <div v-if="loading" class="space-y-4">
        <div v-for="idx in 3" :key="idx" class="h-16 animate-pulse rounded-xl border bg-muted"></div>
      </div>

      <!-- 加载失败 -->
      <Alert v-else-if="loadError" class="mb-5" variant="destructive">
        <AlertDescription>{{ loadError }}</AlertDescription>
        <div class="mt-3">
          <Button size="sm" variant="outline" @click="load">
            {{ t('personalCenter.common.loadRetry') }}
          </Button>
        </div>
      </Alert>

      <template v-else-if="data">
        <!-- 我的邀请码 -->
        <div class="mb-4 rounded-2xl border bg-secondary/40 p-5">
          <p class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            {{ t('personalCenter.invitation.myCode') }}
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <span class="text-3xl font-black tracking-[0.18em] text-foreground">{{ data.invite_code || '—' }}</span>
            <Button type="button" variant="outline" size="sm" @click="copy('code', inviteCodeText)">
              <component :is="copiedKey === 'code' ? Check : Copy" class="h-3.5 w-3.5" />
              {{ copiedKey === 'code' ? t('personalCenter.invitation.copied') : t('personalCenter.invitation.copy') }}
            </Button>
          </div>
        </div>

        <!-- 邀请链接 -->
        <div class="mb-4 rounded-2xl border p-5">
          <p class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            {{ t('personalCenter.invitation.inviteUrl') }}
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-3">
            <span class="min-w-0 flex-1 break-all font-mono text-sm text-foreground">{{ inviteUrlText || '—' }}</span>
            <Button type="button" variant="outline" size="sm" @click="copy('url', inviteUrlText)">
              <component :is="copiedKey === 'url' ? Check : Copy" class="h-3.5 w-3.5" />
              {{ copiedKey === 'url' ? t('personalCenter.invitation.copied') : t('personalCenter.invitation.copy') }}
            </Button>
          </div>
        </div>

        <!-- 下级信息：上级 / 直接邀请人数 / 绑定时间 -->
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div class="rounded-2xl border p-5">
            <p class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              {{ t('personalCenter.invitation.myInviter') }}
            </p>
            <p class="mt-2 text-lg font-bold text-foreground">
              {{ data.inviter_display_name || t('personalCenter.invitation.noInviter') }}
            </p>
          </div>

          <div class="rounded-2xl border p-5">
            <p class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              {{ t('personalCenter.invitation.directCount') }}
            </p>
            <p class="mt-2 text-lg font-bold tabular-nums text-foreground">
              {{ data.direct_invite_count ?? 0 }}
            </p>
          </div>

          <div class="rounded-2xl border p-5">
            <p class="text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
              {{ t('personalCenter.invitation.boundAt') }}
            </p>
            <p class="mt-2 text-sm font-semibold text-foreground">
              {{ boundAtText }}
            </p>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { invitationAPI, type MyInvitationData } from '../../api'
import { copyText } from '../../utils/clipboard'
import { Share2, Copy, Check } from 'lucide-vue-next'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

const loading = ref(false)
const loadError = ref('')
const data = ref<MyInvitationData | null>(null)
const copiedKey = ref<'' | 'code' | 'url'>('')

const inviteCodeText = computed(() => data.value?.invite_code || '')
const inviteUrlText = computed(() => {
  const raw = data.value?.invite_url
  if (!raw) return ''
  try {
    return new URL(raw, window.location.origin).href
  } catch {
    return raw
  }
})

const boundAtText = computed(() => {
  const raw = data.value?.invite_bound_at
  if (!raw) return '—'
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
})

const load = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const response = await invitationAPI.me()
    data.value = (response.data.data || null) as MyInvitationData | null
  } catch (err: any) {
    data.value = null
    loadError.value = err?.message || t('personalCenter.invitation.loadFailed')
  } finally {
    loading.value = false
  }
}

const copy = async (key: 'code' | 'url', value: string) => {
  if (!value) return
  try {
    await copyText(value)
    copiedKey.value = key
    window.setTimeout(() => {
      if (copiedKey.value === key) copiedKey.value = ''
    }, 1600)
  } catch {
    // 复制失败保持静默，用户可手动选择复制
  }
}

onMounted(load)
</script>

<style scoped>
.invitation-panel-enter {
  animation: invitation-panel-enter 0.45s ease both;
}

@keyframes invitation-panel-enter {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
</style>

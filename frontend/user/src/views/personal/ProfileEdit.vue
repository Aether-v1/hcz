<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.profile.title') }}</h1>
    </div>

    <!-- 头像 -->
    <div class="flex flex-col items-center py-6">
      <div class="grid h-20 w-20 place-items-center rounded-full bg-accent text-2xl font-bold text-accent-foreground">
        {{ profileInitial }}
      </div>
      <p class="mt-3 text-sm text-muted-foreground">{{ t('personalCenter.profile.avatarHint') }}</p>
    </div>

    <!-- 资料表单 -->
    <div class="space-y-4 rounded-2xl border bg-card p-5 shadow-sm">
      <!-- 昵称 -->
      <div>
        <Label class="mb-1.5 block text-sm">{{ t('personalCenter.profile.nicknameLabel') }}</Label>
        <Input
          v-model="nickname"
          type="text"
          :placeholder="t('personalCenter.profile.nicknamePlaceholder')"
          class="h-10 text-sm"
          :disabled="saving"
        />
      </div>

      <!-- 邮箱 -->
      <div>
        <Label class="mb-1.5 block text-sm">{{ t('personalCenter.profile.emailLabel') }}</Label>
        <div class="flex h-10 items-center rounded-md border bg-muted px-3 text-sm text-muted-foreground">
          {{ profile.profile?.email || '-' }}
          <span v-if="profile.profile?.email_verified_at" class="ml-2 rounded-full bg-success/10 px-2 py-0.5 text-[10px] font-semibold text-success">
            {{ t('personalCenter.profile.verified') }}
          </span>
        </div>
      </div>

      <!-- 用户ID -->
      <div>
        <Label class="mb-1.5 block text-sm">{{ t('personalCenter.profile.userId') }}</Label>
        <div class="flex h-10 items-center rounded-md border bg-muted px-3 text-sm text-muted-foreground">
          {{ profile.profile?.id || '-' }}
        </div>
      </div>

      <!-- 错误提示 -->
      <div v-if="profile.profileError" class="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive">
        {{ profile.profileError }}
      </div>

      <!-- 保存按钮 -->
      <Button
        type="button"
        class="h-11 w-full text-sm font-bold"
        :disabled="saving || !nickname.trim()"
        @click="handleSave"
      >
        {{ saving ? t('personalCenter.profile.saving') : t('personalCenter.profile.save') }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useUserProfileStore } from '../../stores/userProfile'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

const { t } = useI18n()
const profile = useUserProfileStore()

const nickname = ref('')
const saving = computed(() => profile.savingProfile)

const profileInitial = computed(() => {
  const name = nickname.value.trim() || profile.displayName.trim()
  return name ? name.slice(0, 1).toUpperCase() : 'U'
})

onMounted(async () => {
  if (!profile.profile) {
    await profile.loadProfile()
  }
  nickname.value = profile.profile?.nickname || ''
})

const handleSave = async () => {
  const ok = await profile.saveProfile({ nickname: nickname.value.trim() })
  if (ok) {
    nickname.value = profile.profile?.nickname || ''
  }
}
</script>

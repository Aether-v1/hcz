<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { c2cAPI, type C2CUserStatus } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Card, CardContent } from '@/components/ui/card'
import { formatDate } from '@/utils/format'
import { notifySuccess, notifyError } from '@/utils/notify'

const userIdInput = ref('')
const loading = ref(false)
const user = ref<C2CUserStatus | null>(null)

const disableDialogOpen = ref(false)
const disableReason = ref('')
const submitting = ref(false)

const searchUser = async () => {
  const id = Number(userIdInput.value.trim())
  if (!id || Number.isNaN(id)) {
    notifyError('请输入有效的用户ID')
    return
  }
  loading.value = true
  try {
    const res = await c2cAPI.getUserStatus(id)
    user.value = res.data.data
  } catch {
    user.value = null
  } finally {
    loading.value = false
  }
}

const openDisable = () => {
  disableReason.value = ''
  disableDialogOpen.value = true
}

const doDisable = async () => {
  if (!disableReason.value.trim()) {
    notifyError('请输入禁用原因')
    return
  }
  submitting.value = true
  try {
    await c2cAPI.disableUser(user.value!.user_id, disableReason.value.trim())
    notifySuccess('已禁用该用户的 C2C 功能')
    disableDialogOpen.value = false
    await searchUser()
  } catch {
    // 错误已统一提示
  } finally {
    submitting.value = false
  }
}

const doEnable = async () => {
  submitting.value = true
  try {
    await c2cAPI.enableUser(user.value!.user_id)
    notifySuccess('已恢复该用户的 C2C 功能')
    await searchUser()
  } catch {
    // 错误已统一提示
  } finally {
    submitting.value = false
  }
}

onMounted(() => {})
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">用户 C2C 管理</h1>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div class="w-full sm:w-64">
          <Input v-model="userIdInput" placeholder="输入用户ID查询" @keyup.enter="searchUser" />
        </div>
        <Button :disabled="loading" @click="searchUser">查询</Button>
      </div>
    </div>

    <div v-if="loading" class="rounded-xl border border-border bg-card p-8 text-center text-muted-foreground">加载中…</div>

    <Card v-else-if="user">
      <CardContent class="pt-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="space-y-2">
            <div class="text-lg font-semibold">用户 #{{ user.user_id }}</div>
            <div v-if="user.email" class="text-sm text-muted-foreground">{{ user.email }}</div>
            <div v-if="user.display_name" class="text-sm text-muted-foreground">{{ user.display_name }}</div>
            <div class="mt-2">
              <span
                class="inline-flex rounded-full border px-3 py-1 text-sm"
                :class="user.c2c_banned ? 'border-destructive/30 bg-destructive/10 text-destructive' : 'border-success/30 bg-success/10 text-success'"
              >
                {{ user.c2c_banned ? 'C2C 已禁用' : 'C2C 正常' }}
              </span>
            </div>
            <div v-if="user.c2c_banned && user.ban_reason" class="mt-2 text-sm text-destructive">
              禁用原因：{{ user.ban_reason }}
            </div>
            <div v-if="user.banned_at" class="text-xs text-muted-foreground">
              禁用时间：{{ formatDate(user.banned_at) }}
            </div>
          </div>
          <div class="flex gap-2">
            <Button v-if="!user.c2c_banned" variant="destructive" :disabled="submitting" @click="openDisable">禁用 C2C</Button>
            <Button v-else :disabled="submitting" @click="doEnable">启用 C2C</Button>
          </div>
        </div>
      </CardContent>
    </Card>

    <div v-else class="rounded-xl border border-dashed border-border p-8 text-center text-sm text-muted-foreground">
      输入用户ID查询其 C2C 状态
    </div>

    <Dialog v-model:open="disableDialogOpen">
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>禁用用户 C2C</DialogTitle>
          <DialogDescription>禁用后该用户将无法发布挂单或进行 C2C 交易。</DialogDescription>
        </DialogHeader>
        <div class="space-y-2 py-2">
          <Label class="block text-xs text-muted-foreground">禁用原因（必填）</Label>
          <Textarea v-model="disableReason" rows="3" placeholder="请填写禁用原因" />
        </div>
        <DialogFooter>
          <Button variant="outline" :disabled="submitting" @click="disableDialogOpen = false">取消</Button>
          <Button variant="destructive" :disabled="submitting || !disableReason.trim()" @click="doDisable">确认禁用</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>

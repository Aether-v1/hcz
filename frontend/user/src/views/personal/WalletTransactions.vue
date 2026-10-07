<template>
  <div class="space-y-4 pb-8">
    <!-- 标题 -->
    <div class="mb-2">
      <h1 class="text-xl font-bold tracking-tight text-foreground md:text-2xl">{{ t('personalCenter.wallet.bills') }}</h1>
    </div>

    <WalletTransactionList
      :loading="loading"
      :error="transactionError"
      :transactions="transactions"
      :current-page="pagination.page"
      :total-pages="pagination.total_page"
      @refresh="loadTransactions(pagination.page)"
      @change-page="changePage"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { walletAPI } from '../../api'
import WalletTransactionList from '../../components/wallet/WalletTransactionList.vue'

const { t } = useI18n()

const loading = ref(true)
const transactionError = ref(false)
const transactions = ref<any[]>([])
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })

const loadTransactions = async (page = 1) => {
  loading.value = true
  transactionError.value = false
  try {
    const response = await walletAPI.transactions({
      page,
      page_size: pagination.value.page_size,
    })
    transactions.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    transactions.value = []
    transactionError.value = true
  } finally {
    loading.value = false
  }
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  loadTransactions(page)
}

onMounted(() => {
  loadTransactions(1)
})
</script>

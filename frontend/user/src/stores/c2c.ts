import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
    c2cAPI,
    type C2CListing,
    type C2CTrade,
    type C2CPaymentMethod,
    type C2CWallet,
    type C2CListingMarketFilter,
    type C2CMyListingFilter,
    type C2CTradeListFilter,
    C2C_TRADE_TERMINAL_STATUSES,
} from '../api/c2c'
import { useUserAuthStore } from './userAuth'

interface PaginationState {
    page: number
    page_size: number
    total: number
    total_page: number
}

const DEFAULT_PAGINATION: PaginationState = { page: 1, page_size: 20, total: 0, total_page: 0 }

/**
 * C2C 共享 Store（classic / vault 共用，业务逻辑只此一份）。
 *
 * 管理：market 列表分页 / current trade / my trades / my listings / payment methods / wallet
 */
export const useC2CStore = defineStore('c2c', () => {
    const auth = useUserAuthStore()

    // ─── Wallet ──────────────────────────────────────────────
    const wallet = ref<C2CWallet | null>(null)
    const walletLoading = ref(false)

    const fetchWallet = async () => {
        walletLoading.value = true
        try {
            const res = await c2cAPI.getWallet()
            const data = res.data?.data || {}
            wallet.value = {
                available_balance: String(data.available_balance ?? '0'),
                frozen_balance: String(data.frozen_balance ?? '0'),
                total_balance: String(data.total_balance ?? '0'),
                currency: String(data.currency ?? 'USDT'),
            }
        } finally {
            walletLoading.value = false
        }
    }

    // ─── Payment Methods ─────────────────────────────────────
    const paymentMethods = ref<C2CPaymentMethod[]>([])
    const paymentMethodsLoading = ref(false)

    const enabledPaymentMethods = computed(() =>
        paymentMethods.value.filter((pm) => pm.enabled),
    )

    const hasEnabledPaymentMethod = computed(() => enabledPaymentMethods.value.length > 0)

    const fetchPaymentMethods = async () => {
        paymentMethodsLoading.value = true
        try {
            const res = await c2cAPI.listPaymentMethods()
            paymentMethods.value = (res.data?.data || []) as C2CPaymentMethod[]
        } finally {
            paymentMethodsLoading.value = false
        }
    }

    // ─── Market Listings ─────────────────────────────────────
    const marketListings = ref<C2CListing[]>([])
    const marketLoading = ref(false)
    const marketPagination = ref<PaginationState>({ ...DEFAULT_PAGINATION })
    const marketFilter = ref<C2CListingMarketFilter>({ page: 1, page_size: 20 })

    const fetchMarketListings = async (filter?: C2CListingMarketFilter) => {
        if (filter) {
            marketFilter.value = { ...marketFilter.value, ...filter }
        }
        marketLoading.value = true
        try {
            const res = await c2cAPI.listMarketListings(marketFilter.value)
            marketListings.value = (res.data?.data || []) as C2CListing[]
            const pg = res.data?.pagination
            if (pg) {
                marketPagination.value = {
                    page: Number(pg.page) || 1,
                    page_size: Number(pg.page_size) || 20,
                    total: Number(pg.total) || 0,
                    total_page: Number(pg.total_page) || 0,
                }
            }
        } finally {
            marketLoading.value = false
        }
    }

    // ─── My Listings ─────────────────────────────────────────
    const myListings = ref<C2CListing[]>([])
    const myListingsLoading = ref(false)
    const myListingsPagination = ref<PaginationState>({ ...DEFAULT_PAGINATION })
    const myListingsFilter = ref<C2CMyListingFilter>({ page: 1, page_size: 20 })

    const fetchMyListings = async (filter?: C2CMyListingFilter) => {
        if (filter) {
            myListingsFilter.value = { ...myListingsFilter.value, ...filter }
        }
        myListingsLoading.value = true
        try {
            const res = await c2cAPI.listMyListings(myListingsFilter.value)
            myListings.value = (res.data?.data || []) as C2CListing[]
            const pg = res.data?.pagination
            if (pg) {
                myListingsPagination.value = {
                    page: Number(pg.page) || 1,
                    page_size: Number(pg.page_size) || 20,
                    total: Number(pg.total) || 0,
                    total_page: Number(pg.total_page) || 0,
                }
            }
        } finally {
            myListingsLoading.value = false
        }
    }

    // ─── My Trades ───────────────────────────────────────────
    const myTrades = ref<C2CTrade[]>([])
    const myTradesLoading = ref(false)
    const myTradesPagination = ref<PaginationState>({ ...DEFAULT_PAGINATION })
    const myTradesFilter = ref<C2CTradeListFilter>({ page: 1, page_size: 20 })

    const fetchMyTrades = async (filter?: C2CTradeListFilter) => {
        if (filter) {
            myTradesFilter.value = { ...myTradesFilter.value, ...filter }
        }
        myTradesLoading.value = true
        try {
            const res = await c2cAPI.listMyTrades(myTradesFilter.value)
            myTrades.value = (res.data?.data || []) as C2CTrade[]
            const pg = res.data?.pagination
            if (pg) {
                myTradesPagination.value = {
                    page: Number(pg.page) || 1,
                    page_size: Number(pg.page_size) || 20,
                    total: Number(pg.total) || 0,
                    total_page: Number(pg.total_page) || 0,
                }
            }
        } finally {
            myTradesLoading.value = false
        }
    }

    // ─── Current Trade ───────────────────────────────────────
    const currentTrade = ref<C2CTrade | null>(null)
    const currentTradeLoading = ref(false)

    const currentUserRole = computed<'buyer' | 'seller' | null>(() => {
        if (!currentTrade.value || !auth.user?.id) return null
        const uid = Number(auth.user.id)
        if (currentTrade.value.buyer_user_id === uid) return 'buyer'
        if (currentTrade.value.seller_user_id === uid) return 'seller'
        return null
    })

    const isCurrentTradeTerminal = computed(() =>
        currentTrade.value
            ? (C2C_TRADE_TERMINAL_STATUSES as string[]).includes(currentTrade.value.status)
            : false,
    )

    const fetchCurrentTrade = async (id: number) => {
        currentTradeLoading.value = true
        try {
            const res = await c2cAPI.getTradeDetail(id)
            currentTrade.value = (res.data?.data || null) as C2CTrade | null
        } finally {
            currentTradeLoading.value = false
        }
    }

    const clearCurrentTrade = () => {
        currentTrade.value = null
    }

    // ─── Reset ───────────────────────────────────────────────
    const reset = () => {
        wallet.value = null
        paymentMethods.value = []
        marketListings.value = []
        myListings.value = []
        myTrades.value = []
        currentTrade.value = null
    }

    return {
        // wallet
        wallet,
        walletLoading,
        fetchWallet,
        // payment methods
        paymentMethods,
        paymentMethodsLoading,
        enabledPaymentMethods,
        hasEnabledPaymentMethod,
        fetchPaymentMethods,
        // market
        marketListings,
        marketLoading,
        marketPagination,
        marketFilter,
        fetchMarketListings,
        // my listings
        myListings,
        myListingsLoading,
        myListingsPagination,
        myListingsFilter,
        fetchMyListings,
        // my trades
        myTrades,
        myTradesLoading,
        myTradesPagination,
        myTradesFilter,
        fetchMyTrades,
        // current trade
        currentTrade,
        currentTradeLoading,
        currentUserRole,
        isCurrentTradeTerminal,
        fetchCurrentTrade,
        clearCurrentTrade,
        // utils
        reset,
    }
})

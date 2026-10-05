import { createRouter, createWebHistory } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { useAppStore } from '../stores/app'
import { useTelegramMiniAppStore } from '../stores/telegramMiniApp'
import { captureAffiliateFromRoute } from '../utils/affiliate'
import { templateView } from '../templates/registry'
import { GOOGLE_REDIRECT_FRONTEND_CALLBACK_PATH } from '../utils/googleRedirect'

type RouteComponentLoader = () => Promise<unknown>

const homeViewLoader: RouteComponentLoader = () => import('../views/Home.vue')
const productsViewLoader: RouteComponentLoader = () => import('../views/Products.vue')
const productDetailViewLoader: RouteComponentLoader = () => import('../views/ProductDetail.vue')
const cartViewLoader: RouteComponentLoader = () => import('../views/Cart.vue')
const checkoutViewLoader: RouteComponentLoader = () => import('../views/Checkout.vue')
const paymentViewLoader: RouteComponentLoader = () => import('../views/Payment.vue')
const blogViewLoader: RouteComponentLoader = () => import('../views/Blog.vue')
const noticeViewLoader: RouteComponentLoader = () => import('../views/Notice.vue')
const loginViewLoader: RouteComponentLoader = () => import('../views/auth/Login.vue')
const resellerLayoutLoader: RouteComponentLoader = () => import('../views/reseller/ResellerConsoleLayout.vue')

const routeWarmupLoaders: RouteComponentLoader[] = [
    productsViewLoader,
    productDetailViewLoader,
    cartViewLoader,
    checkoutViewLoader,
    paymentViewLoader,
    blogViewLoader,
    noticeViewLoader,
    loginViewLoader,
]

let hasScheduledRouteWarmup = false

const shouldWarmupRoutes = () => {
    if (typeof window === 'undefined' || typeof navigator === 'undefined') {
        return false
    }

    const connection = (navigator as Navigator & {
        connection?: {
            saveData?: boolean
            effectiveType?: string
        }
    }).connection

    if (connection?.saveData) {
        return false
    }

    return connection?.effectiveType !== 'slow-2g' && connection?.effectiveType !== '2g'
}

const scheduleIdleTask = (task: () => void) => {
    if (typeof window === 'undefined') {
        return
    }

    if ('requestIdleCallback' in window && typeof window.requestIdleCallback === 'function') {
        window.requestIdleCallback(task, { timeout: 1500 })
        return
    }

    window.setTimeout(task, 600)
}

const runRouteWarmupQueue = (loaders: RouteComponentLoader[]) => {
    const nextLoader = loaders.shift()
    if (!nextLoader || typeof window === 'undefined') {
        return
    }

    void nextLoader()
        .catch(() => undefined)
        .finally(() => {
            window.setTimeout(() => {
                scheduleIdleTask(() => runRouteWarmupQueue(loaders))
            }, 400)
        })
}

export const warmupCommonRoutes = () => {
    if (hasScheduledRouteWarmup || !shouldWarmupRoutes()) {
        return
    }

    hasScheduledRouteWarmup = true

    const startWarmup = () => {
        window.setTimeout(() => {
            scheduleIdleTask(() => runRouteWarmupQueue([...routeWarmupLoaders]))
        }, 1200)
    }

    if (document.readyState === 'complete') {
        startWarmup()
        return
    }

    window.addEventListener('load', startWarmup, { once: true })
}

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    scrollBehavior(to, _from, savedPosition) {
        if (savedPosition) {
            return savedPosition
        }
        if (to.hash) {
            return { el: to.hash, top: 80 }
        }
        return { top: 0 }
    },
    routes: [
        {
            path: '/',
            name: 'home',
            component: templateView('Home', homeViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/products',
            name: 'products',
            component: templateView('Products', productsViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/categories/:slug',
            name: 'category-products',
            component: templateView('Products', productsViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/products/:slug',
            name: 'product-detail',
            component: templateView('ProductDetail', productDetailViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/cart',
            name: 'cart',
            component: templateView('Cart', cartViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/checkout',
            name: 'checkout',
            component: templateView('Checkout', checkoutViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/pay',
            name: 'payment',
            component: templateView('Payment', paymentViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/guest/orders/:order_no?',
            redirect: '/me/orders',
        },
        {
            path: '/me',
            name: 'personal-center',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'overview' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/profile',
            name: 'personal-center-profile',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'profile' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/security',
            name: 'personal-center-security',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'security' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/orders',
            name: 'personal-center-orders',
            component: templateView('Orders', () => import('../views/Orders.vue')),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/wallet',
            name: 'personal-center-wallet',
            component: templateView('Wallet', () => import('../views/Wallet.vue')),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/wallet/withdrawal',
            name: 'wallet-withdrawal',
            component: () => import('../views/WalletAction.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/wallet/withdrawal-history',
            name: 'wallet-withdrawal-history',
            component: () => import('../views/WalletAction.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/invitation',
            name: 'personal-center-invitation',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'invitation' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/notifications',
            name: 'notifications',
            component: () => import('../views/Notifications.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c',
            name: 'c2c-home',
            component: () => import('../views/c2c/C2CHome.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/buy',
            name: 'c2c-buy',
            component: () => import('../views/c2c/BuyUSDT.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/sell',
            name: 'c2c-sell',
            component: () => import('../views/c2c/SellUSDT.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/my-listings',
            name: 'c2c-my-listings',
            component: () => import('../views/c2c/MyListings.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/listings/:id',
            name: 'c2c-listing-detail',
            component: () => import('../views/c2c/ListingDetail.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/trades',
            name: 'c2c-my-trades',
            component: () => import('../views/c2c/MyTrades.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/trades/:id',
            name: 'c2c-trade-detail',
            component: () => import('../views/c2c/TradeDetail.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/c2c/payment-methods',
            name: 'c2c-payment-methods',
            component: () => import('../views/c2c/PaymentMethods.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/support',
            name: 'support-home',
            component: () => import('../views/support/HelpCenter.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/support/tickets',
            name: 'support-tickets',
            component: () => import('../views/support/Tickets.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/support/tickets/new',
            name: 'support-ticket-new',
            component: () => import('../views/support/NewTicket.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/support/tickets/:id',
            name: 'support-ticket-detail',
            component: () => import('../views/support/TicketDetail.vue'),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/gift-cards',
            name: 'personal-center-gift-cards',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'giftCard' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/api',
            name: 'personal-center-api',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'api' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/affiliate',
            name: 'personal-center-affiliate',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'affiliate' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/reseller',
            name: 'personal-center-reseller',
            redirect: '/reseller',
            meta: { requiresUserAuth: true, resellerConsole: true }
        },
        {
            path: '/reseller',
            component: resellerLayoutLoader,
            meta: { requiresUserAuth: true, resellerConsole: true },
            children: [
                { path: '', name: 'reseller-dashboard', component: () => import('../views/reseller/ResellerDashboard.vue') },
                { path: 'apply', name: 'reseller-apply', component: () => import('../views/reseller/ResellerApply.vue') },
                { path: 'domains', name: 'reseller-domains', component: () => import('../views/reseller/ResellerDomains.vue') },
                { path: 'site', name: 'reseller-site', component: () => import('../views/reseller/ResellerSiteConfig.vue') },
                { path: 'products', name: 'reseller-products', component: () => import('../views/reseller/ResellerProducts.vue') },
                { path: 'orders', name: 'reseller-orders', component: () => import('../views/reseller/ResellerOrders.vue') },
                { path: 'orders/:order_no', name: 'reseller-order-detail', component: () => import('../views/reseller/ResellerOrderDetail.vue') },
                { path: 'finance', name: 'reseller-finance', component: () => import('../views/reseller/ResellerFinance.vue') },
                { path: 'ledger', name: 'reseller-ledger', component: () => import('../views/reseller/ResellerLedger.vue') },
                { path: 'withdraws', name: 'reseller-withdraws', component: () => import('../views/reseller/ResellerWithdraws.vue') },
            ],
        },
        {
            path: '/orders/:order_no',
            name: 'order-detail',
            component: templateView('OrderDetail', () => import('../views/OrderDetail.vue')),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/recharge-orders/:recharge_no',
            name: 'recharge-order-detail',
            component: templateView('RechargeOrderDetail', () => import('../views/RechargeOrderDetail.vue')),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/blog',
            name: 'blog',
            component: templateView('Blog', blogViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/blog/:slug',
            name: 'blog-detail',
            component: templateView('BlogDetail', () => import('../views/BlogDetail.vue')),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/notice',
            name: 'notice',
            component: templateView('Notice', noticeViewLoader),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/about',
            name: 'about',
            component: templateView('About', () => import('../views/About.vue')),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/discovery',
            name: 'discovery',
            component: templateView('Discovery', () => import('../views/Discovery.vue')),
            meta: { requiresUserAuth: true },
        },
        {
            path: '/terms',
            name: 'terms',
            component: templateView('Legal', () => import('../views/Legal.vue')),
            props: { type: 'terms' }
        },
        {
            path: '/privacy',
            name: 'privacy',
            component: templateView('Legal', () => import('../views/Legal.vue')),
            props: { type: 'privacy' }
        },
        {
            path: '/auth/login',
            name: 'user-login',
            component: templateView('auth/Login', loginViewLoader),
            meta: { userGuest: true }
        },
        {
            path: '/auth/register',
            name: 'user-register',
            component: templateView('auth/Register', () => import('../views/auth/Register.vue')),
            meta: { userGuest: true }
        },
        {
            path: '/auth/forgot',
            name: 'user-forgot',
            component: templateView('auth/Forgot', () => import('../views/auth/Forgot.vue')),
            meta: { userGuest: true }
        },
        {
            path: '/auth/telegram/callback',
            name: 'user-telegram-callback',
            component: templateView('auth/TelegramCallback', () => import('../views/auth/TelegramCallback.vue')),
        },
        {
            path: GOOGLE_REDIRECT_FRONTEND_CALLBACK_PATH,
            name: 'user-google-callback',
            component: templateView('auth/GoogleCallback', () => import('../views/auth/GoogleCallback.vue')),
        },
        {
            path: '/:pathMatch(.*)*',
            name: 'not-found',
            component: templateView('NotFound', () => import('../views/NotFound.vue')),
        },
    ],
})

// Navigation Guard
router.beforeEach(async (to, _from, next) => {
    const userAuthStore = useUserAuthStore()
    const appStore = useAppStore()
    void captureAffiliateFromRoute(to)

    // Ensure config is loaded before checking template mode
    if (!appStore.config) {
        await appStore.loadConfig()
    }

    // 后端已停用游客支付；旧支付链接不能继续落入 /guest/* 请求分支。
    if (to.path === '/pay' && to.query.guest) {
        next('/me/orders')
        return
    }

    if (to.meta.requiresUserAuth) {
        if (!userAuthStore.isAuthenticated) {
            const redirect = encodeURIComponent(to.fullPath)
            next(`/auth/login?redirect=${redirect}`)
        } else if (to.meta.resellerConsole && !appStore.canAccessResellerConsole) {
            next('/me/orders')
        } else {
            next()
        }
    }
    else if (to.meta.userGuest) {
        if (userAuthStore.isAuthenticated) {
            next('/me/orders')
        } else {
            next()
        }
    }
    else {
        next()
    }
})

// Update SEO on route change
router.afterEach(() => {
    const appStore = useAppStore()
    const telegramMiniAppStore = useTelegramMiniAppStore()
    appStore.applySEO()
    telegramMiniAppStore.syncRouteBackButton(router.currentRoute.value.path, () => {
        if (window.history.length > 1) {
            router.back()
            return
        }
        void router.push('/')
    })
})

export default router


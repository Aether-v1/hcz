// Re-export client instances and ApiResponse
export { api, userApi } from './client'
export type { ApiResponse } from './client'

// Re-export all types
export type {
    UserProfileData,
    PublicMemberLevel,
    UpdateUserProfilePayload,
    UserLoginLogItem,
    SendChangeEmailCodePayload,
    ChangeEmailPayload,
    ChangeUserPasswordPayload,
    TelegramAuthPayload,
    TelegramMiniAppAuthPayload,
    TelegramBindingData,
    GoogleCredentialPayload,
    GoogleBindingData,
    WalletAccountData,
    WalletTransactionData,
    WalletRechargePayload,
    WalletRechargeOrderData,
    WalletRechargeResult,
    GiftCardRedeemResult,
    AffiliateDashboardData,
    AffiliateCommissionData,
    AffiliateWithdrawData,
    AffiliateWithdrawApplyPayload,
    ResellerProfileSummaryData,
    ResellerManagementProfileData,
    ResellerDomainData,
    ResellerManagementSnapshotData,
    ResellerApplyPayload,
    ResellerCustomDomainPayload,
    ResellerLocalizedText,
    ResellerSiteConfigPayload,
    ResellerSiteConfigData,
    ResellerSiteConfigSnapshotData,
    ResellerBalanceData,
    ResellerLedgerData,
    ResellerWithdrawData,
    ResellerDashboardData,
    ResellerOrderListParams,
    ResellerOrderStatsParams,
    ResellerOrderData,
    ResellerOrderItemData,
    ResellerOrderDetailData,
    ResellerOrderStatsData,
    ResellerWithdrawApplyPayload,
    CreatePaymentPayload,
    PaymentCreateResult,
    CaptchaPayload,
} from './types'

// Re-export domain APIs
export { productAPI, postAPI, bannerAPI, categoryAPI, memberLevelAPI } from './product'
export { userAuthAPI, captchaAPI, configAPI } from './auth'
export { userProfileAPI } from './user'
export { userOrderAPI, guestOrderAPI, paymentAPI } from './order'
export { walletAPI, giftCardAPI } from './wallet'
export { notificationAPI } from './notification'
export { c2cAPI } from './c2c'
export type {
    C2CPaymentMethod,
    C2CListing,
    C2CTrade,
    C2CDispute,
    C2CWallet,
    C2CTradeStatus,
    C2CListingStatus,
} from './c2c'
export { affiliateAPI } from './affiliate'
export { resellerAPI } from './reseller'
export { apiCredentialAPI } from './credential'
export { invitationAPI, type MyInvitationData } from './invitation'

// Default export for backward compatibility
export { default } from './client'


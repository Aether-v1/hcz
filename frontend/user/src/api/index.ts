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
    AffiliateTransferRecord,
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
export { userOrderAPI, paymentAPI } from './order'
export { walletAPI, giftCardAPI } from './wallet'
export { notificationAPI } from './notification'
export { supportAPI } from './support'
export type {
    SupportCategory,
    SupportTicketSummary,
    SupportTicketDetail,
    SupportMessage,
    SupportAttachment,
    SupportTicketListData,
    SupportTicketDetailData,
    CreateTicketPayload,
    CreateTicketResult,
    UploadAttachmentResult,
    CreateReplyPayload,
} from '../types/support'
export { invitationAPI, type MyInvitationData } from './invitation'
export { affiliateAPI, type AffiliateApplicationData } from './affiliate'
export { resellerAPI } from './reseller'
export { apiCredentialAPI } from './credential'

// Default export for backward compatibility
export { default } from './client'


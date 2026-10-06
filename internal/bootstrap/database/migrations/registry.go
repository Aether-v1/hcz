package migrations

import (
	"errors"
	"fmt"
	"time"

	"github.com/Aether-v1/hcz/internal/constants"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	apicredentialdomain "github.com/Aether-v1/hcz/internal/modules/apicredential/domain"
	auditlogdomain "github.com/Aether-v1/hcz/internal/modules/auditlog/domain"
	c2cdomain "github.com/Aether-v1/hcz/internal/modules/c2c/domain"
	cardsecretdomain "github.com/Aether-v1/hcz/internal/modules/cardsecret/domain"
	cartdomain "github.com/Aether-v1/hcz/internal/modules/cart/domain"
	categorydomain "github.com/Aether-v1/hcz/internal/modules/catalog/category/domain"
	mappingdomain "github.com/Aether-v1/hcz/internal/modules/catalog/mapping/domain"
	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"
	channelclientdomain "github.com/Aether-v1/hcz/internal/modules/channelclient/domain"
	contentdomain "github.com/Aether-v1/hcz/internal/modules/content/domain"
	coupondomain "github.com/Aether-v1/hcz/internal/modules/coupon/domain"
	downstreamcallbackdomain "github.com/Aether-v1/hcz/internal/modules/downstreamcallback/domain"
	fulfillmentdomain "github.com/Aether-v1/hcz/internal/modules/fulfillment/domain"
	giftcarddomain "github.com/Aether-v1/hcz/internal/modules/giftcard/domain"
	admindomain "github.com/Aether-v1/hcz/internal/modules/identity/admin/domain"
	emailverificationdomain "github.com/Aether-v1/hcz/internal/modules/identity/emailverification/domain"
	externalidentitydomain "github.com/Aether-v1/hcz/internal/modules/identity/externalidentity/domain"
	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	memberleveldomain "github.com/Aether-v1/hcz/internal/modules/memberlevel/domain"
	notificationdomain "github.com/Aether-v1/hcz/internal/modules/notification/domain"
	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"
	orderriskcontract "github.com/Aether-v1/hcz/internal/modules/orderrisk/contract"
	orderriskdomain "github.com/Aether-v1/hcz/internal/modules/orderrisk/domain"
	paymentdomain "github.com/Aether-v1/hcz/internal/modules/payment/domain"
	procurementdomain "github.com/Aether-v1/hcz/internal/modules/procurement/domain"
	promotiondomain "github.com/Aether-v1/hcz/internal/modules/promotion/domain"
	reconciliationdomain "github.com/Aether-v1/hcz/internal/modules/reconciliation/domain"
	resellerstore "github.com/Aether-v1/hcz/internal/modules/reseller/infrastructure/gormstore"
	settingsstore "github.com/Aether-v1/hcz/internal/modules/settings/infrastructure/gormstore"
	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	siteconnectiondomain "github.com/Aether-v1/hcz/internal/modules/siteconnection/domain"
	supportdomain "github.com/Aether-v1/hcz/internal/modules/supportticket/domain"
	broadcastdomain "github.com/Aether-v1/hcz/internal/modules/telegram/broadcast/domain"
	usernotificationdomain "github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"
	withdrawaldomain "github.com/Aether-v1/hcz/internal/modules/walletwithdrawal/domain"
	"github.com/Aether-v1/hcz/internal/platform/database/gormdb"

	"gorm.io/gorm"
)

// AutoMigrate owns the application-wide schema registry and ordered data migrations.
func AutoMigrate() error {
	db := gormdb.DB
	if err := db.AutoMigrate(
		&admindomain.Admin{},
		&userdomain.User{},
		&externalidentitydomain.Identity{},
		&affiliatedomain.Profile{},
		&affiliatedomain.Click{},
		&affiliatedomain.Commission{},
		&affiliatedomain.WithdrawRequest{},
		&affiliatedomain.CommissionLedger{},
		&affiliatedomain.Application{},
		&walletdomain.Account{},
		&walletdomain.Transaction{},
		&walletdomain.RechargeOrder{},
		&withdrawaldomain.Withdrawal{},
		&withdrawaldomain.Address{},
		&auditlogdomain.UserLoginLog{},
		&auditlogdomain.AuthzAuditLog{},
		&notificationdomain.NotificationLog{},
		&auditlogdomain.AdminLoginLog{},
		&emailverificationdomain.Code{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&orderdomain.AfterSaleTicket{},
		&orderriskdomain.LockKey{},
		&cartdomain.Item{},
		&paymentdomain.PaymentChannel{},
		&paymentdomain.Payment{},
		&cardsecretdomain.Secret{},
		&cardsecretdomain.Batch{},
		&giftcarddomain.GiftCard{},
		&giftcarddomain.GiftCardBatch{},
		&fulfillmentdomain.Fulfillment{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&contentdomain.Post{},
		&contentdomain.PostProduct{},
		&contentdomain.PostCategory{},
		&contentdomain.Banner{},
		&settingsstore.SettingRecord{},
		&apicredentialdomain.ApiCredential{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
		&procurementdomain.Order{},
		&downstreamcallbackdomain.OrderRef{},
		&reconciliationdomain.Job{},
		&reconciliationdomain.Item{},
		&channelclientdomain.Client{},
		&broadcastdomain.Broadcast{},
		&memberleveldomain.MemberLevel{},
		&memberleveldomain.MemberLevelPrice{},
		&contentdomain.Media{},
		&usernotificationdomain.UserNotification{},
		&c2cdomain.PaymentMethod{},
		&c2cdomain.Listing{},
		&c2cdomain.Trade{},
		&c2cdomain.Dispute{},
		&c2cdomain.RiskSignal{},
		&supportdomain.Category{},
		&supportdomain.Ticket{},
		&supportdomain.Message{},
		&supportdomain.Attachment{},
		&supportdomain.Audit{},
		&sitebuilderdomain.HomeEntry{},
		&sitebuilderdomain.DiscoveryBlock{},
		&sitebuilderdomain.SiteAuditLog{},
	); err != nil {
		return err
	}

	if err := ensureUserIDSequenceStart(db); err != nil {
		return err
	}
	if err := ensureUserOAuthIdentityUserProviderUniqueIndex(); err != nil {
		return err
	}
	if err := backfillPendingOrderRiskIPs(db); err != nil {
		return err
	}
	if err := resellerstore.Migrate(db); err != nil {
		return err
	}
	if err := migrateCartSKUUniqueIndex(); err != nil {
		return err
	}
	if err := ensureProductSKUMigration(); err != nil {
		return err
	}
	if err := ensureManualStockRemainingMigration(); err != nil {
		return err
	}
	if err := ensureCategoryParentMigration(); err != nil {
		return err
	}
	if err := ensurePaymentProviderBepusdtRenameMigration(); err != nil {
		return err
	}
	if err := ensurePaymentChannelBepusdtConfigMigration(); err != nil {
		return err
	}
	if err := ensurePaymentFeePolicyMigration(); err != nil {
		return err
	}
	if err := ensureOrderRefundPaymentFeeMigration(); err != nil {
		return err
	}
	if err := ensureOrderItemOriginalPriceMigration(); err != nil {
		return err
	}
	if err := BackfillInviteCodes(db); err != nil {
		return err
	}
	if err := ensureCartForeignKeyConstraints(); err != nil {
		return err
	}
	if err := ensureProcurementOrderForeignKeyConstraint(); err != nil {
		return err
	}
	if err := migrateAffiliateCommissionMultilevel(); err != nil {
		return err
	}
	if err := migrateWalletDualBalance(); err != nil {
		return err
	}
	if db.Migrator().HasColumn(&productdomain.Product{}, "price_currency") {
		if err := db.Migrator().DropColumn(&productdomain.Product{}, "price_currency"); err != nil {
			return err
		}
	}
	if err := SeedSupportTicketCategories(db); err != nil {
		return err
	}
	if err := SeedHomeEntries(db); err != nil {
		return err
	}
	if err := ensureOrderIdempotencyUniqueIndex(); err != nil {
		return err
	}
	if err := ensureAffiliateApplicationPendingUniqueIndex(db); err != nil {
		return err
	}
	if err := migrateGrandfatheredAffiliateApplications(db); err != nil {
		return err
	}
	return nil
}

// ensureUserIDSequenceStart 让 users 表自增主键从 1000 起步。
//
// 幂等且不会回退已有更大的 ID：
//   - PostgreSQL：setval(pg_get_serial_sequence('users','id'), GREATEST(999, MAX(id)), true)。
//     pg_get_serial_sequence 动态获取 sequence 名，避免 schema rename 后写死 users_id_seq。
//     is_called=true 表示下一次 nextval 返回 value+increment（通常 value+1）。
//     空表时 value=999 → 下一个 ID=1000；已有 max=2350 时 value=2350 → 下一个 ID=2351。
//   - SQLite：驱动对主键使用 INTEGER PRIMARY KEY AUTOINCREMENT，当前值持久化在
//     sqlite_sequence。仅当 max(id)<999 时插入一条 id=999 的占位行再硬删除，
//     使 sqlite_sequence.seq=999，下一个自增 ID=1000；已有 ID>=999 时不做任何事。
//
// 调用时机：AutoMigrate 建表之后、其它数据迁移之前。
func ensureUserIDSequenceStart(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&userdomain.User{}) {
		return nil
	}

	// Unscoped：连软删除行一起算入 max，避免软删占用过的 ID 导致 sequence 回退。
	var maxID int64
	if err := db.Unscoped().Model(&userdomain.User{}).
		Select("COALESCE(MAX(id), 0)").Scan(&maxID).Error; err != nil {
		return err
	}

	switch db.Dialector.Name() {
	case "postgres":
		// pg_get_serial_sequence 动态获取 users.id 对应的 sequence 名（如 public.users_id_seq）。
		// setval(..., true)：is_called=true，下一次 nextval 返回 value+1。
		// 空表 max=0 → value=999 → 下一 ID=1000；已有 max=2350 → value=2350 → 下一 ID=2351。
		return db.Exec(
			`SELECT setval(pg_get_serial_sequence('users', 'id')::regclass, GREATEST(999, ?), true)`, maxID,
		).Error
	case "sqlite":
		if maxID >= 999 {
			return nil
		}
		// 占位行满足 NOT NULL 约束后硬删除；AUTOINCREMENT 让 sqlite_sequence 记住 999，
		// 下一次插入得到 1000。email 带时间戳避免唯一索引冲突。
		seed := userdomain.User{
			Email:        fmt.Sprintf("__seq_seed_%d__@local.invalid", time.Now().UnixNano()),
			PasswordHash: "__seq_seed__",
		}
		seed.ID = 999
		if err := db.Create(&seed).Error; err != nil {
			// id=999 已被占用（maxID<999 时不应发生），视为无需调整。
			return nil
		}
		return db.Unscoped().Delete(&userdomain.User{}, 999).Error
	}
	return nil
}

// SeedSupportTicketCategories 写入预置工单分类（FirstOrCreate，可重复执行）。
func SeedSupportTicketCategories(db *gorm.DB) error {
	seeds := []supportdomain.Category{
		{Code: "account", Name: "账户问题", Enabled: true, SortOrder: 1, DefaultPriority: "normal"},
		{Code: "recharge", Name: "充值问题", Enabled: true, SortOrder: 2, DefaultPriority: "normal"},
		{Code: "wallet", Name: "钱包问题", Enabled: true, SortOrder: 3, DefaultPriority: "high"},
		{Code: "withdrawal", Name: "提现问题", Enabled: true, SortOrder: 4, DefaultPriority: "high"},
		{Code: "c2c", Name: "C2C 交易", Enabled: true, SortOrder: 5, DefaultPriority: "high"},
		{Code: "affiliate", Name: "推广返利", Enabled: true, SortOrder: 6, DefaultPriority: "normal"},
		{Code: "technical", Name: "技术故障", Enabled: true, SortOrder: 7, DefaultPriority: "normal"},
		{Code: "other", Name: "其他问题", Enabled: true, SortOrder: 99, DefaultPriority: "normal"},
	}
	for i := range seeds {
		var existing supportdomain.Category
		err := db.Where("code = ?", seeds[i].Code).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := db.Create(&seeds[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedHomeEntries 写入预置首页入口（FirstOrCreate，可重复执行）。
// 使用 key 唯一约束，已存在的 key 跳过，不覆盖 Admin 的自定义改动。
func SeedHomeEntries(db *gorm.DB) error {
	seeds := []sitebuilderdomain.HomeEntry{
		{
			Key: "entry_recharge", Title: "生活充值", Icon: "recharge",
			ActionType: "internal", ActionTarget: "recharge",
			Recommended: true, Enabled: true, SortOrder: 1,
		},
		{
			Key: "entry_c2c", Title: "C2C 交易", Icon: "c2c",
			ActionType: "internal", ActionTarget: "c2c", Badge: "新",
			Enabled: true, SortOrder: 2,
		},
		{
			Key: "entry_wallet", Title: "Wallet", Icon: "wallet",
			ActionType: "internal", ActionTarget: "wallet",
			Enabled: true, SortOrder: 3,
		},
		{
			Key: "entry_invitation", Title: "邀请中心", Icon: "invitation",
			ActionType: "internal", ActionTarget: "invitation",
			Enabled: true, SortOrder: 4,
		},
	}
	for i := range seeds {
		var existing sitebuilderdomain.HomeEntry
		err := db.Where("key = ?", seeds[i].Key).First(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := db.Create(&seeds[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
func backfillPendingOrderRiskIPs(db *gorm.DB) error {
	type orderIPRow struct {
		ID       uint
		ClientIP string
	}
	var rows []orderIPRow
	if err := db.Model(&orderdomain.Order{}).
		Select("id", "client_ip").
		Where("deleted_at IS NULL AND status = ? AND (risk_ip IS NULL OR risk_ip = '') AND client_ip <> ''", constants.OrderStatusPendingPayment).
		Find(&rows).Error; err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			riskIP := orderriskcontract.NormalizeRiskIP(row.ClientIP)
			if riskIP == "" {
				continue
			}
			if err := tx.Model(&orderdomain.Order{}).
				Where("id = ? AND (risk_ip IS NULL OR risk_ip = '')", row.ID).
				UpdateColumn("risk_ip", riskIP).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ensureAffiliateApplicationPendingUniqueIndex 创建部分唯一索引，
// 防止同一用户存在多条 pending application（并发兜底）。
// 幂等：先 DROP INDEX IF EXISTS 再 CREATE。
func ensureAffiliateApplicationPendingUniqueIndex(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Dialector.Name() != "postgres" {
		// SQLite 不支持 partial index WHERE 子句的标准语法，
		// 但 GORM/SQLite 会通过部分唯一索引的变体处理；这里跳过非 postgres。
		return nil
	}
	if !db.Migrator().HasTable(&affiliatedomain.Application{}) {
		return nil
	}
	if err := db.Exec(`DROP INDEX IF EXISTS idx_affiliate_apps_user_pending`).Error; err != nil {
		return err
	}
	return db.Exec(
		`CREATE UNIQUE INDEX idx_affiliate_apps_user_pending ON affiliate_applications(user_id) WHERE status = 'pending'`,
	).Error
}

// migrateGrandfatheredAffiliateApplications 为现有 active Profile 创建 approved application 历史记录。
// 幂等：已存在 application 的 user_id 跳过。disabled profile 不创建。
func migrateGrandfatheredAffiliateApplications(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if !db.Migrator().HasTable(&affiliatedomain.Application{}) || !db.Migrator().HasTable(&affiliatedomain.Profile{}) {
		return nil
	}

	// 查询所有 active profile，且对应用户没有任何 application 记录。
	var profiles []affiliatedomain.Profile
	if err := db.Where("status = ? AND deleted_at IS NULL", constants.AffiliateProfileStatusActive).
		Find(&profiles).Error; err != nil {
		return err
	}
	if len(profiles) == 0 {
		return nil
	}

	// 收集所有 user_id
	userIDs := make([]uint, 0, len(profiles))
	profileByUserID := make(map[uint]affiliatedomain.Profile, len(profiles))
	for _, p := range profiles {
		userIDs = append(userIDs, p.UserID)
		profileByUserID[p.UserID] = p
	}

	// 查询已有 application 的 user_id
	var existingUserIDs []uint
	if err := db.Model(&affiliatedomain.Application{}).
		Where("user_id IN ?", userIDs).
		Distinct("user_id").
		Pluck("user_id", &existingUserIDs).Error; err != nil {
		return err
	}
	existingSet := make(map[uint]struct{}, len(existingUserIDs))
	for _, id := range existingUserIDs {
		existingSet[id] = struct{}{}
	}

	// 构造待插入的 application 列表
	var toCreate []affiliatedomain.Application
	for _, uid := range userIDs {
		if _, ok := existingSet[uid]; ok {
			continue
		}
		p := profileByUserID[uid]
		reviewedAt := p.CreatedAt
		toCreate = append(toCreate, affiliatedomain.Application{
			UserID:     uid,
			Status:     constants.AffiliateAppStatusApproved,
			Reason:     "migration",
			ReviewNote: "grandfathered: legacy auto-open",
			ReviewedBy: 0,
			ReviewedAt: &reviewedAt,
		})
	}

	if len(toCreate) == 0 {
		return nil
	}

	// 批量插入，每批 100 条
	return db.CreateInBatches(toCreate, 100).Error
}

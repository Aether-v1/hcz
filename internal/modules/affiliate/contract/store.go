package contract

import (
	"time"

	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	productdomain "github.com/Aether-v1/hcz/internal/modules/catalog/product/domain"

	"github.com/Aether-v1/hcz/internal/modules/affiliate/domain"
	settingsintegration "github.com/Aether-v1/hcz/internal/modules/settings/schema/integration"
	"github.com/shopspring/decimal"
)

// OrderReader 是 Affiliate 计算订单佣金所需的最小订单读取端口。
type OrderReader interface {
	GetByID(id uint) (*orderdomain.Order, error)
}

// ProductReader 是 Affiliate 计算可返利金额所需的最小商品读取端口。
type ProductReader interface {
	ListByIDs(ids []uint) ([]productdomain.Product, error)
}

// SettingsReader 是 Affiliate 所需的动态设置读取端口。
type SettingsReader interface {
	GetAffiliateSetting() (settingsintegration.AffiliateSetting, error)
}

type ProfileListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
	Code     string
	Keyword  string
}

type CommissionListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	OrderID            uint
	OrderNo            string
	Status             string
	Keyword            string
	Level              int
	CreatedFrom        *time.Time
	CreatedTo          *time.Time
}

type WithdrawListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	Status             string
	Keyword            string
	CreatedFrom        *time.Time
	CreatedTo          *time.Time
}

type ProfileStatsAggregate struct {
	ClickCount          int64
	ValidOrderCount     int64
	PendingCommission   decimal.Decimal
	AvailableCommission decimal.Decimal
	WithdrawnCommission decimal.Decimal
}

type Store interface {
	WithinTransaction(fn func(Store) error) error

	GetProfileByID(id uint) (*domain.Profile, error)
	UpdateProfileStatus(id uint, status string, updatedAt time.Time) error
	BatchUpdateProfileStatus(ids []uint, status string, updatedAt time.Time) (int64, error)
	GetProfileByUserID(userID uint) (*domain.Profile, error)
	GetProfileByCode(code string) (*domain.Profile, error)
	CreateProfile(profile *domain.Profile) error
	ListProfiles(filter ProfileListFilter) ([]domain.Profile, int64, error)

	CreateClick(click *domain.Click) error
	HasRecentClick(profileID uint, visitorKey, landingPath string, since time.Time) (bool, error)
	GetLatestActiveProfileByVisitorKey(visitorKey string, since time.Time) (*domain.Profile, error)
	CountClicksByProfile(profileID uint) (int64, error)

	GetCommissionByOrderAndProfile(orderID, profileID uint, commissionType string) (*domain.Commission, error)
	// GetCommissionByOrderBeneficiaryLevel 是多级别幂等检查：同一订单同一收益人同一层级仅允许一条。
	GetCommissionByOrderBeneficiaryLevel(orderID, beneficiaryUserID uint, level int) (*domain.Commission, error)
	CreateCommission(commission *domain.Commission) error
	// BatchCreateCommissions 批量插入多级别佣金；调用方需在事务内使用并自行处理唯一冲突。
	BatchCreateCommissions(commissions []*domain.Commission) error
	UpdateCommission(commission *domain.Commission) error
	ListCommissions(filter CommissionListFilter) ([]domain.Commission, int64, error)
	ListCommissionsByOrder(orderID uint, statuses []string) ([]domain.Commission, error)
	ListCommissionsByOrderForUpdate(orderID uint, statuses []string) ([]domain.Commission, error)
	ListCommissionsByWithdrawIDForUpdate(withdrawID uint) ([]domain.Commission, error)
	// MarkPendingCommissionsAvailable 将到期的 pending_confirm 佣金转 available，并返回本次实际转换的佣金列表（用于发送到账通知）。
	MarkPendingCommissionsAvailable(before, now time.Time) ([]domain.Commission, error)
	CountValidOrdersByProfile(profileID uint) (int64, error)
	SumCommissionByProfile(profileID uint, statuses []string, unboundOnly bool) (decimal.Decimal, error)
	ListAvailableCommissionsForUpdate(profileID uint) ([]domain.Commission, error)
	BatchUpdateCommissions(ids []uint, updates map[string]interface{}) error
	// GetProfilesByUserIDs 批量查询用户对应的推广档案（用于多级别资格检查）。
	GetProfilesByUserIDs(userIDs []uint) ([]domain.Profile, error)

	CreateWithdraw(request *domain.WithdrawRequest) error
	UpdateWithdraw(request *domain.WithdrawRequest) error
	GetWithdrawByID(id uint) (*domain.WithdrawRequest, error)
	GetWithdrawByIDForUpdate(id uint) (*domain.WithdrawRequest, error)
	ListWithdraws(filter WithdrawListFilter) ([]domain.WithdrawRequest, int64, error)
	GetProfileStatsBatch(profileIDs []uint) (map[uint]ProfileStatsAggregate, error)
}

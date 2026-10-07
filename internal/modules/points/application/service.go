package application

import (
	"strings"

	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
)

type Options struct {
	Repository   pointscontract.Repository
	Transactions pointscontract.UnitOfWork
	// Stats 是运营统计所需的只读聚合端口（P4）。
	// 未注入时 GetStats 返回 ErrStatsSourceRequired，其余积分用例不受影响。
	Stats pointscontract.StatsPorts
}

// Service 是积分模块的应用服务。所有积分变更必须经由核心 mutation 入口
// （applyMutation）完成：锁账户 → 校验 policy → 计算 → 同事务写账户 + Ledger。
type Service struct {
	repository   pointscontract.Repository
	transactions pointscontract.UnitOfWork
	stats        pointscontract.StatsPorts
}

var _ pointscontract.UseCase = (*Service)(nil)

func NewService(options Options) *Service {
	return &Service{
		repository:   options.Repository,
		transactions: options.Transactions,
		stats:        options.Stats,
	}
}

func cleanReason(raw, fallback string) string {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return fallback
	}
	return reason
}

// validateReason 校验 reason 长度（列宽 255）。
// 不做静默截断：截断会让审计看到与实际意图不符的文本。
func validateReason(reason string) error {
	if len([]rune(reason)) > pointscontract.MaxReasonLength {
		return pointscontract.ErrReasonTooLong
	}
	return nil
}

// validateReference 校验 reference 长度（列宽 120，含派生前缀）。
// Idempotency-Key 由客户端提交，超长必须显式拒绝，不能指望 DB 报错。
func validateReference(reference string) error {
	if len([]rune(reference)) > pointscontract.MaxReferenceLength {
		return pointscontract.ErrReferenceTooLong
	}
	return nil
}

// validateAmountBounds 校验单笔金额的业务硬上限（挡住后台误输入的天文数字）。
// int64 层面的溢出保护在 applyMutation 内独立执行，两者互不替代。
func validateAmountBounds(amount int64) error {
	if amount <= 0 {
		return pointscontract.ErrInvalidAmount
	}
	if amount > pointscontract.MaxPointsAmount {
		return pointscontract.ErrAmountTooLarge
	}
	return nil
}

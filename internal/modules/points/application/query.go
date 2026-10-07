package application

import (
	pointscontract "github.com/Aether-v1/hcz/internal/modules/points/contract"
	pointsdomain "github.com/Aether-v1/hcz/internal/modules/points/domain"
)

// GetAccount 返回用户积分账户。
// 从未产生积分的用户返回 (nil, nil)，由 HTTP 层归一化为零值账户
// （balance/total_earned/total_spent = 0），不返回 404；
// 且查询是只读路径，不因 GET 请求创建账户（首次积分 mutation 时才创建）。
func (s *Service) GetAccount(userID uint) (*pointsdomain.Account, error) {
	if userID == 0 {
		return nil, pointscontract.ErrAccountNotFound
	}
	return s.repository.GetAccountByUserID(userID)
}

// ListLedgerEntries 分页查询积分流水（append-only，按创建时间倒序）。
// 过滤条件（user_id / action_type / source_type / reference / 时间区间 / 收支方向）
// 全部在 store 层参数化实现；排序固定 created_at DESC, id DESC，不接受客户端指定字段。
func (s *Service) ListLedgerEntries(filter pointscontract.LedgerListFilter) ([]pointsdomain.LedgerEntry, int64, error) {
	return s.repository.ListLedgerEntries(filter)
}

// ListAccounts 分页查询积分账户（P4：balance<0 排查、运营核对）。
// 负余额不做任何钳制——它是退款冲正/Admin 扣减的真实结果，必须原样可见。
func (s *Service) ListAccounts(filter pointscontract.AccountListFilter) ([]pointscontract.AccountWithUser, int64, error) {
	return s.repository.ListAccounts(filter)
}

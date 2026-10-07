package application

import (
	"strings"

	"github.com/Aether-v1/hcz/internal/constants"
	walletcontract "github.com/Aether-v1/hcz/internal/modules/wallet/contract"
	walletdomain "github.com/Aether-v1/hcz/internal/modules/wallet/domain"

	"github.com/shopspring/decimal"
)

func (s *Service) Recharge(input walletcontract.RechargeInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if input.UserID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	amount := input.Amount.Decimal.Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, nil, walletcontract.ErrInvalidAmount
	}
	return s.changeBalance(
		input.UserID, amount, constants.WalletTxnTypeRecharge, nil,
		uniqueReference("recharge", input.UserID),
		cleanRemark(input.Remark, "用户充值"),
		normalizeCurrency(input.Currency),
		nil,
	)
}

func (s *Service) AdminAdjustBalance(input walletcontract.AdjustBalanceInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	if input.UserID == 0 || input.OperatorAdminID == 0 {
		return nil, nil, walletcontract.ErrAccountNotFound
	}
	delta := input.Delta.Decimal.Round(2)
	if delta.IsZero() {
		return nil, nil, walletcontract.ErrInvalidAmount
	}

	currency := normalizeCurrency(input.Currency)
	remark := cleanRemark(input.Remark, "管理员调整余额")
	operatorID := input.OperatorAdminID

	// 期望的方向与金额（与 changeBalance 内部写库口径一致），用于幂等命中比对。
	direction := constants.WalletTxnDirectionIn
	amount := delta
	if delta.Sign() < 0 {
		direction = constants.WalletTxnDirectionOut
		amount = delta.Abs().Round(2)
	}

	// 幂等：调用方提供 Reference（Idempotency-Key）时先查重。
	// 命中且参数一致 → 直接返回原流水（HTTP 200，不重复入账）；
	// 命中但参数不一致 → 返回 ErrIdempotencyConflict（HTTP 409）。
	// 并发下的最后防线是 wallet_transactions.reference 唯一索引。
	if ref := strings.TrimSpace(input.Reference); ref != "" {
		existing, err := s.repository.GetTransactionByReference(ref)
		if err != nil {
			return nil, nil, err
		}
		if existing != nil {
			if existing.UserID == input.UserID &&
				existing.Direction == direction &&
				existing.Currency == currency &&
				existing.Amount.Decimal.Equal(amount) {
				acct, acctErr := s.repository.GetAccountByUserID(input.UserID)
				if acctErr != nil {
					return nil, nil, acctErr
				}
				return acct, existing, nil
			}
			return nil, nil, walletcontract.ErrIdempotencyConflict
		}
		return s.changeBalance(
			input.UserID, delta, constants.WalletTxnTypeAdminAdjust, nil,
			ref, remark, currency, &operatorID,
		)
	}

	return s.changeBalance(
		input.UserID, delta, constants.WalletTxnTypeAdminAdjust, nil,
		uniqueReference("admin_adjust", input.UserID),
		remark, currency, &operatorID,
	)
}

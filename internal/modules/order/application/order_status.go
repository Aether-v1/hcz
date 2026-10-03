package application

import (
	"time"

	orderdomain "github.com/Aether-v1/hcz/internal/modules/order/domain"

	"github.com/Aether-v1/hcz/internal/constants"
	ordermachine "github.com/Aether-v1/hcz/internal/modules/order/application/ordermachine"
	ordercontract "github.com/Aether-v1/hcz/internal/modules/order/contract"
)

// SyncParentStatus 汇总父订单状态并写入
func SyncParentStatus(orderStore ordercontract.Store, parentID uint, now time.Time) (string, error) {
	if parentID == 0 {
		return "", nil
	}
	parent, err := orderStore.GetByID(parentID)
	if err != nil {
		return "", err
	}
	if parent == nil || parent.ParentID != nil {
		return "", nil
	}
	if parent.Status == constants.OrderStatusCanceled {
		return parent.Status, nil
	}
	newStatus := CalcParentStatus(parent.Children, parent.Status)
	if newStatus == "" || newStatus == parent.Status {
		return parent.Status, nil
	}
	updates := map[string]interface{}{
		"updated_at": now,
	}
	if err := orderStore.UpdateStatus(parent.ID, newStatus, updates); err != nil {
		return "", err
	}
	return newStatus, nil
}

// CalcParentStatus HCZ M1: 基于五态聚合子订单状态计算父订单状态。
// 规则（优先级从高到低）：
//  1. 所有子 canceled → 父 canceled
//  2. 任一子 failed → 父 failed
//  3. 任一子 pending_recharge → 父 pending_recharge
//  4. 任一子 processing → 父 processing
//  5. 所有子 completed → 父 completed
//  6. refund_status 不影响主状态聚合
func CalcParentStatus(children []orderdomain.Order, currentStatus string) string {
	if len(children) == 0 {
		return currentStatus
	}

	// HCZ M1: 先将每个子订单状态归一为五态，再聚合
	var completedCount, canceledCount, failedCount, pendingRechargeCount, processingCount int
	for _, child := range children {
		normalized := ordermachine.Normalize(child.Status).Status
		switch normalized {
		case constants.OrderStatusCompleted:
			completedCount++
		case constants.OrderStatusCanceled:
			canceledCount++
		case constants.OrderStatusFailed:
			failedCount++
		case constants.OrderStatusPendingRecharge:
			pendingRechargeCount++
		case constants.OrderStatusProcessing:
			processingCount++
		}
	}

	// 1. 全部取消 → 父取消
	if canceledCount == len(children) {
		return constants.OrderStatusCanceled
	}
	// 2. 任一失败 → 父失败
	if failedCount > 0 {
		return constants.OrderStatusFailed
	}
	// 3. 任一待充值 → 父待充值
	if pendingRechargeCount > 0 {
		return constants.OrderStatusPendingRecharge
	}
	// 4. 任一处理中 → 父处理中
	if processingCount > 0 {
		return constants.OrderStatusProcessing
	}
	// 5. 全部完成 → 父完成
	if completedCount == len(children) {
		return constants.OrderStatusCompleted
	}
	return currentStatus
}

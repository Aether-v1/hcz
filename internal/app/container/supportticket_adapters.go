package container

import (
	"context"

	supportcontract "github.com/Aether-v1/hcz/internal/modules/supportticket/contract"
	usernotificationcontract "github.com/Aether-v1/hcz/internal/modules/usernotification/contract"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// supportUserLookup 函数式适配器，由容器注入具体读取函数。
type supportUserLookup struct {
	getByID func(userID uint) (id uint, email, displayName string, err error)
}

func (a *supportUserLookup) GetByID(userID uint) (uint, string, string, error) {
	return a.getByID(userID)
}

type supportAdminLookup struct {
	getAdminByID func(adminID uint) (id uint, username string, err error)
}

func (a *supportAdminLookup) GetAdminByID(adminID uint) (uint, string, error) {
	return a.getAdminByID(adminID)
}

// supportNotifierAdapter 把 UserNotificationService 适配为工单 NotificationCreator。
type supportNotifierAdapter struct {
	creator usernotificationcontract.Creator
}

func (a *supportNotifierAdapter) CreateNotification(ctx context.Context, in supportcontract.NotificationInput) error {
	if a == nil || a.creator == nil {
		return nil
	}
	return a.creator.CreateNotification(ctx, usernotificationcontract.CreateInput{
		UserID:  in.UserID,
		Type:    in.Type,
		Title:   in.Title,
		Body:    in.Body,
		Data:    jsonmap.JSON{},
		BizType: in.BizType,
		BizID:   in.BizID,
	})
}

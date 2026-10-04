package usernotificationhttp

import (
	"time"

	"github.com/Aether-v1/hcz/internal/modules/usernotification/domain"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// notificationDTO 是用户通知对外 DTO，不直接暴露 GORM model。
type notificationDTO struct {
	ID        uint         `json:"id"`
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Body      string       `json:"body"`
	Data      jsonmap.JSON `json:"data"`
	BizType   string       `json:"biz_type"`
	BizID     uint         `json:"biz_id"`
	IsRead    bool         `json:"is_read"`
	ReadAt    *time.Time   `json:"read_at"`
	CreatedAt time.Time    `json:"created_at"`
}

func newNotificationDTO(n *domain.UserNotification) notificationDTO {
	if n == nil {
		return notificationDTO{}
	}
	data := n.Data
	if data == nil {
		data = jsonmap.JSON{}
	}
	return notificationDTO{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Body:      n.Body,
		Data:      data,
		BizType:   n.BizType,
		BizID:     n.BizID,
		IsRead:    n.IsRead,
		ReadAt:    n.ReadAt,
		CreatedAt: n.CreatedAt,
	}
}

func newNotificationList(items []domain.UserNotification) []notificationDTO {
	result := make([]notificationDTO, 0, len(items))
	for i := range items {
		result = append(result, newNotificationDTO(&items[i]))
	}
	return result
}

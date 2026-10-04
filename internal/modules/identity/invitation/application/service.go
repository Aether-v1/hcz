package application

import (
	"errors"
	"strings"
	"time"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
	settingsapp "github.com/Aether-v1/hcz/internal/modules/settings/application"
)

// ErrInvitationNotFound 目标用户不存在。
var ErrInvitationNotFound = errors.New("invitation user not found")

// MyInvitation 是 GET /api/v1/invitation/me 的返回体。
type MyInvitation struct {
	InviteCode         string     `json:"invite_code"`
	InviteURL          string     `json:"invite_url"`
	InviterCode        *string    `json:"inviter_code"`         // 无上级为 null
	InviterDisplayName *string    `json:"inviter_display_name"` // 无上级为 null，不暴露完整邮箱
	DirectInviteCount  int64      `json:"direct_invite_count"`
	InviteBoundAt      *time.Time `json:"invite_bound_at"`
}

// UserReader 邀请查询所需的用户读取端口。
type UserReader interface {
	GetByID(uint) (*userdomain.User, error)
	CountDirectInvitees(inviterID uint) (int64, error)
}

// BrandReader 提供站点根地址，用于拼接邀请链接。
type BrandReader interface {
	GetSiteBrand() (settingsapp.SiteBrand, error)
}

// Service 邀请绑定查询服务（只读，不涉及多级返利）。
type Service struct {
	users UserReader
	brand BrandReader
}

// NewService 创建邀请查询服务。
func NewService(users UserReader, brand BrandReader) *Service {
	return &Service{users: users, brand: brand}
}

// GetMyInvitation 返回当前登录用户的邀请信息。IDOR 安全：只根据登录态 userID 查询自己。
func (s *Service) GetMyInvitation(userID uint) (MyInvitation, error) {
	if s == nil || s.users == nil || userID == 0 {
		return MyInvitation{}, ErrInvitationNotFound
	}
	user, err := s.users.GetByID(userID)
	if err != nil {
		return MyInvitation{}, err
	}
	if user == nil {
		return MyInvitation{}, ErrInvitationNotFound
	}

	directCount, err := s.users.CountDirectInvitees(userID)
	if err != nil {
		return MyInvitation{}, err
	}

	resp := MyInvitation{
		InviteCode:        user.InviteCode,
		DirectInviteCount: directCount,
		InviteBoundAt:     user.InviteBoundAt,
	}

	// 上级信息（可空）。
	if user.InviterID != nil && *user.InviterID != 0 {
		inviter, err := s.users.GetByID(*user.InviterID)
		if err != nil {
			return MyInvitation{}, err
		}
		if inviter != nil {
			code := inviter.InviteCode
			name := displayNameOf(inviter)
			resp.InviterCode = &code
			resp.InviterDisplayName = &name
		}
	}

	resp.InviteURL = s.buildInviteURL(user.InviteCode)
	return resp, nil
}

// displayNameOf 返回对外展示名：优先昵称，缺省时取邮箱前缀，不暴露完整邮箱。
func displayNameOf(u *userdomain.User) string {
	if name := strings.TrimSpace(u.DisplayName); name != "" {
		return name
	}
	if at := strings.Index(u.Email, "@"); at > 0 {
		return u.Email[:at]
	}
	return u.Email
}

// buildInviteURL 拼接站点注册邀请链接。站点根地址未配置时回退为相对路径。
func (s *Service) buildInviteURL(code string) string {
	base := ""
	if s.brand != nil {
		if brand, err := s.brand.GetSiteBrand(); err == nil {
			base = strings.TrimRight(strings.TrimSpace(brand.SiteURL), "/")
		}
	}
	return base + "/register?invite=" + code
}

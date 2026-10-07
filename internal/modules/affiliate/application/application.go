package application

import (
	"strings"
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/constants"
)

// ApplyAffiliate 用户提交推广申请（划转资格审批）。
// 新架构：profile 可能因懒创建已存在（commission anchor），申请状态以 affiliate_applications 为真源。
// 返回创建的 pending application。并发安全依赖 DB partial unique index。
func (s *Service) ApplyAffiliate(userID uint, reason string) (*affiliatedomain.Application, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	if s.repo == nil || s.userRepo == nil {
		return nil, ErrNotFound
	}

	// 检查推广设置是否开启
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, ErrDisabled
	}

	// 检查用户存在且未禁用
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(user.Status) == constants.UserStatusDisabled {
		return nil, ErrUserDisabled
	}

	// 检查 profile 是否被管理员禁用（风控独立于申请流程）
	existingProfile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existingProfile != nil && strings.TrimSpace(existingProfile.Status) == constants.AffiliateProfileStatusDisabled {
		return nil, ErrDisabled
	}

	// 申请状态真源：affiliate_applications，不再依赖 profile 是否存在
	latestApp, err := s.repo.GetLatestApplicationByUserID(userID)
	if err != nil {
		return nil, err
	}
	if latestApp != nil {
		switch strings.TrimSpace(latestApp.Status) {
		case constants.AffiliateAppStatusApproved:
			return nil, ErrAlreadyActive
		case constants.AffiliateAppStatusPending:
			return nil, ErrApplicationPending
		}
		// rejected：允许重新申请，继续创建新 pending application
	}

	// 二次确认无 pending application（DB partial unique index 兜底并发）
	pendingApp, err := s.repo.GetPendingApplicationByUserID(userID)
	if err != nil {
		return nil, err
	}
	if pendingApp != nil {
		return nil, ErrApplicationPending
	}

	app := &affiliatedomain.Application{
		UserID: userID,
		Status: constants.AffiliateAppStatusPending,
		Reason: strings.TrimSpace(reason),
	}
	if err := s.repo.CreateApplication(app); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrApplicationPending
		}
		return nil, err
	}

	s.recordAudit("affiliate_apply", userID, userID, map[string]interface{}{
		"application_id": app.ID,
		"reason":         app.Reason,
	})
	return app, nil
}

// GetUserApplication 返回当前用户最新的申请记录（无则返回 nil, nil）。
func (s *Service) GetUserApplication(userID uint) (*affiliatedomain.Application, error) {
	if userID == 0 || s.repo == nil {
		return nil, nil
	}
	return s.repo.GetLatestApplicationByUserID(userID)
}

// ApproveApplication 管理员通过申请（仅解锁划转资格，不补发历史佣金）。
// 在事务中执行：行锁 → 验证 pending → GetOrCreate profile → 更新 application → 写审计。
func (s *Service) ApproveApplication(applicationID uint, adminID uint) (*affiliatedomain.Profile, error) {
	if applicationID == 0 || s.repo == nil {
		return nil, ErrApplicationNotFound
	}

	var result *affiliatedomain.Profile
	err := s.repo.WithinTransaction(func(tx affiliatecontract.Store) error {
		app, err := tx.GetApplicationByIDForUpdate(applicationID)
		if err != nil {
			return err
		}
		if app == nil {
			return ErrApplicationNotFound
		}
		if strings.TrimSpace(app.Status) != constants.AffiliateAppStatusPending {
			return ErrApplicationAlreadyReviewed
		}

		// GetOrCreate：profile 可能因懒创建已存在（commission anchor）
		profile, err := s.getOrCreateProfileInTx(tx, app.UserID)
		if err != nil {
			return err
		}
		// 管理员不得通过申请静默恢复已禁用的 profile（风控独立）
		if strings.TrimSpace(profile.Status) == constants.AffiliateProfileStatusDisabled {
			return ErrProfileDisabledCannotApprove
		}

		now := time.Now()
		if err := tx.UpdateApplicationStatus(app.ID, constants.AffiliateAppStatusApproved, "approved", adminID, now); err != nil {
			return err
		}

		result = profile
		s.recordAudit("affiliate_approve", adminID, app.UserID, map[string]interface{}{
			"application_id": app.ID,
			"profile_id":     profile.ID,
			"before_status":  app.Status,
			"after_status":   constants.AffiliateAppStatusApproved,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// RejectApplication 管理员拒绝申请。
// 在事务中执行：行锁 → 验证 pending → 更新 application → 写审计。不创建 profile。
func (s *Service) RejectApplication(applicationID uint, adminID uint, reason string) error {
	if applicationID == 0 || s.repo == nil {
		return ErrApplicationNotFound
	}

	return s.repo.WithinTransaction(func(tx affiliatecontract.Store) error {
		app, err := tx.GetApplicationByIDForUpdate(applicationID)
		if err != nil {
			return err
		}
		if app == nil {
			return ErrApplicationNotFound
		}
		if strings.TrimSpace(app.Status) != constants.AffiliateAppStatusPending {
			return ErrApplicationAlreadyReviewed
		}

		now := time.Now()
		if err := tx.UpdateApplicationStatus(app.ID, constants.AffiliateAppStatusRejected, strings.TrimSpace(reason), adminID, now); err != nil {
			return err
		}

		s.recordAudit("affiliate_reject", adminID, app.UserID, map[string]interface{}{
			"application_id": app.ID,
			"reason":         strings.TrimSpace(reason),
			"before_status":  app.Status,
			"after_status":   constants.AffiliateAppStatusRejected,
		})
		return nil
	})
}

// GetApplicationDetail 按 ID 查询申请详情（含 user 信息）。
func (s *Service) GetApplicationDetail(applicationID uint) (*affiliatedomain.Application, error) {
	if applicationID == 0 || s.repo == nil {
		return nil, ErrApplicationNotFound
	}
	app, err := s.repo.GetApplicationByID(applicationID)
	if err != nil {
		return nil, err
	}
	if app == nil {
		return nil, ErrApplicationNotFound
	}
	return app, nil
}

// ListAdminApplications 管理端申请列表。
func (s *Service) ListAdminApplications(filter AffiliateApplicationListFilter) ([]affiliatedomain.Application, int64, error) {
	if s.repo == nil {
		return []affiliatedomain.Application{}, 0, nil
	}
	return s.repo.ListApplications(affiliatecontract.ApplicationListFilter{
		Page:        filter.Page,
		PageSize:    filter.PageSize,
		UserID:      filter.UserID,
		Status:      strings.TrimSpace(filter.Status),
		Keyword:     strings.TrimSpace(filter.Keyword),
		CreatedFrom: filter.CreatedFrom,
		CreatedTo:   filter.CreatedTo,
	})
}

// GetUserProfileForGateway 返回当前用户的 profile（供 /affiliate/profile 端点）。
func (s *Service) GetUserProfileForGateway(userID uint) (*affiliatedomain.Profile, error) {
	if userID == 0 || s.repo == nil {
		return nil, nil
	}
	return s.repo.GetProfileByUserID(userID)
}

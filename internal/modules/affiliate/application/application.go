package application

import (
	"strings"
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/constants"
)

// ApplyAffiliate 用户提交推广申请。
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

	// 检查是否已有 active profile
	existingProfile, err := s.repo.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existingProfile != nil {
		if strings.TrimSpace(existingProfile.Status) == constants.AffiliateProfileStatusActive {
			return nil, ErrAlreadyActive
		}
		// disabled profile 不能自行重新申请
		if strings.TrimSpace(existingProfile.Status) == constants.AffiliateProfileStatusDisabled {
			return nil, ErrDisabled
		}
	}

	// 检查是否已有 pending application（DB partial unique index 兜底并发）
	pendingApp, err := s.repo.GetPendingApplicationByUserID(userID)
	if err != nil {
		return nil, err
	}
	if pendingApp != nil {
		return nil, ErrApplicationPending
	}

	// rejected 用户允许重新申请（创建新 pending application）
	app := &affiliatedomain.Application{
		UserID: userID,
		Status: constants.AffiliateAppStatusPending,
		Reason: strings.TrimSpace(reason),
	}
	if err := s.repo.CreateApplication(app); err != nil {
		// 并发兜底：partial unique index 冲突
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

// ApproveApplication 管理员通过申请。
// 在事务中执行：行锁 → 验证 pending → 创建/激活 profile → 更新 application → 写审计。
func (s *Service) ApproveApplication(applicationID uint, adminID uint) (*affiliatedomain.Profile, error) {
	if applicationID == 0 || s.repo == nil {
		return nil, ErrApplicationNotFound
	}

	var result *affiliatedomain.Profile
	err := s.repo.WithinTransaction(func(tx affiliatecontract.Store) error {
		// 行锁查询 application（tx 是事务绑定的 store）
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

		// 幂等保护：检查用户是否已有 active profile
		existingProfile, err := tx.GetProfileByUserID(app.UserID)
		if err != nil {
			return err
		}
		if existingProfile != nil && strings.TrimSpace(existingProfile.Status) == constants.AffiliateProfileStatusActive {
			// 已有 active profile，直接更新 application 为 approved 并返回
			now := time.Now()
			if err := tx.UpdateApplicationStatus(app.ID, constants.AffiliateAppStatusApproved, "approved", adminID, now); err != nil {
				return err
			}
			result = existingProfile
			return nil
		}

		// 创建新 profile（复用 generateAffiliateCode）
		profile, err := s.createActiveProfileInTx(tx, app.UserID)
		if err != nil {
			return err
		}

		// 更新 application 状态
		now := time.Now()
		if err := tx.UpdateApplicationStatus(app.ID, constants.AffiliateAppStatusApproved, "approved", adminID, now); err != nil {
			return err
		}

		result = profile
		s.recordAudit("affiliate_approve", adminID, app.UserID, map[string]interface{}{
			"application_id": app.ID,
			"profile_id":      profile.ID,
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

// ---- 内部辅助方法 ----

// createActiveProfileInTx 在事务内创建 active profile（复用 generateAffiliateCode）。
func (s *Service) createActiveProfileInTx(tx affiliatecontract.Store, userID uint) (*affiliatedomain.Profile, error) {
	const maxRetry = 8
	for i := 0; i < maxRetry; i++ {
		code, genErr := generateAffiliateCode()
		if genErr != nil {
			return nil, genErr
		}
		profile := &affiliatedomain.Profile{
			UserID:        userID,
			AffiliateCode: code,
			Status:        constants.AffiliateProfileStatusActive,
		}
		if err := tx.CreateProfile(profile); err != nil {
			if isUniqueViolation(err) {
				continue
			}
			return nil, err
		}
		created, err := tx.GetProfileByID(profile.ID)
		if err != nil {
			return nil, err
		}
		if created != nil {
			return created, nil
		}
		return profile, nil
	}
	return nil, ErrCodeInvalid
}

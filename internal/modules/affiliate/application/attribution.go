package application

import (
	"strings"
	"time"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/constants"
)

// ResolveOrderAffiliateSnapshot 解析下单归因快照（最近30天最后一次有效点击优先）
func (s *Service) ResolveOrderAffiliateSnapshot(userID uint, rawCode, rawVisitorKey string) (*uint, string, error) {
	code := affiliatedomain.NormalizeCode(rawCode)
	visitorKey := strings.TrimSpace(rawVisitorKey)
	if s.repo == nil {
		return nil, "", nil
	}

	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return nil, "", err
	}
	if !setting.Enabled {
		return nil, "", nil
	}

	if visitorKey != "" {
		profile, err := s.repo.GetLatestActiveProfileByVisitorKey(visitorKey, time.Now().Add(-affiliateAttributionWindow))
		if err != nil {
			return nil, "", err
		}
		if profile != nil {
			if userID > 0 && profile.UserID == userID {
				return nil, "", nil
			}
			profileID := profile.ID
			return &profileID, profile.AffiliateCode, nil
		}
	}

	if code == "" {
		return nil, "", nil
	}

	profile, err := s.repo.GetProfileByCode(code)
	if err != nil {
		return nil, "", err
	}
	if profile == nil || strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
		return nil, "", nil
	}
	if userID > 0 && profile.UserID == userID {
		return nil, "", nil
	}

	profileID := profile.ID
	return &profileID, profile.AffiliateCode, nil
}

// TrackClick 记录推广点击
func (s *Service) TrackClick(input TrackClickInput) error {
	if s.repo == nil {
		return nil
	}
	code := affiliatedomain.NormalizeCode(input.AffiliateCode)
	if code == "" {
		return nil
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return err
	}
	if !setting.Enabled {
		return nil
	}
	profile, err := s.repo.GetProfileByCode(code)
	if err != nil {
		return err
	}
	if profile == nil || strings.TrimSpace(profile.Status) != constants.AffiliateProfileStatusActive {
		return nil
	}
	visitorKey := strings.TrimSpace(input.VisitorKey)
	landingPath := strings.TrimSpace(input.LandingPath)
	if visitorKey != "" {
		duplicated, err := s.repo.HasRecentClick(profile.ID, visitorKey, landingPath, time.Now().Add(-affiliateClickDedupeWindow))
		if err != nil {
			return err
		}
		if duplicated {
			return nil
		}
	}

	click := &affiliatedomain.Click{
		AffiliateProfileID: profile.ID,
		VisitorKey:         visitorKey,
		LandingPath:        landingPath,
		Referrer:           strings.TrimSpace(input.Referrer),
		ClientIP:           strings.TrimSpace(input.ClientIP),
		UserAgent:          strings.TrimSpace(input.UserAgent),
		CreatedAt:          time.Now(),
	}
	return s.repo.CreateClick(click)
}

// ResolveRegistrationInviterUserID 根据访客标识解析注册时的直接上级用户 ID。
// 复用下单归因同一份 click 记录（30 天窗口），仅做读取，不产生佣金、不改写任何状态。
// 无归因 / 推广功能关闭 / 出错时返回 (0, nil)，由注册流程静默降级为无上级。
func (s *Service) ResolveRegistrationInviterUserID(visitorKey string) (uint, error) {
	visitorKey = strings.TrimSpace(visitorKey)
	if s.repo == nil || visitorKey == "" {
		return 0, nil
	}
	setting, err := s.settings.GetAffiliateSetting()
	if err != nil {
		return 0, err
	}
	if !setting.Enabled {
		return 0, nil
	}
	profile, err := s.repo.GetLatestActiveProfileByVisitorKey(visitorKey, time.Now().Add(-affiliateAttributionWindow))
	if err != nil {
		return 0, err
	}
	if profile == nil {
		return 0, nil
	}
	return profile.UserID, nil
}

package application

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	affiliatecontract "github.com/Aether-v1/hcz/internal/modules/affiliate/contract"
	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/constants"
)

// UpdateAffiliateProfileStatus 管理端更新返利用户状态。
// adminID 为操作者（管理员）ID，用于审计留痕；为 0 时审计基础设施会跳过记录。
func (s *Service) UpdateAffiliateProfileStatus(profileID uint, adminID uint, rawStatus string) (*affiliatedomain.Profile, error) {
	if profileID == 0 || s.repo == nil {
		return nil, ErrNotFound
	}
	nextStatus := strings.TrimSpace(rawStatus)
	if nextStatus != constants.AffiliateProfileStatusActive && nextStatus != constants.AffiliateProfileStatusDisabled {
		return nil, ErrProfileStatusInvalid
	}

	profile, err := s.repo.GetProfileByID(profileID)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, ErrNotFound
	}
	beforeStatus := strings.TrimSpace(profile.Status)
	if beforeStatus == nextStatus {
		return profile, nil
	}
	if err := s.repo.UpdateProfileStatus(profileID, nextStatus, time.Now()); err != nil {
		return nil, err
	}
	s.recordAudit("affiliate_profile_status_changed", adminID, profile.UserID, map[string]interface{}{
		"profile_id":    profileID,
		"before_status": beforeStatus,
		"after_status":  nextStatus,
	})
	return s.repo.GetProfileByID(profileID)
}

// BatchUpdateAffiliateProfileStatus 管理端批量更新返利用户状态。
// adminID 为操作者（管理员）ID，用于逐条审计留痕。
func (s *Service) BatchUpdateAffiliateProfileStatus(profileIDs []uint, adminID uint, rawStatus string) (int64, error) {
	if s.repo == nil {
		return 0, ErrNotFound
	}
	nextStatus := strings.TrimSpace(rawStatus)
	if nextStatus != constants.AffiliateProfileStatusActive && nextStatus != constants.AffiliateProfileStatusDisabled {
		return 0, ErrProfileStatusInvalid
	}
	normalizedIDs := normalizeAffiliateProfileIDs(profileIDs)
	if len(normalizedIDs) == 0 {
		return 0, nil
	}
	beforeStatuses := make(map[uint]string, len(normalizedIDs))
	for _, id := range normalizedIDs {
		profile, err := s.repo.GetProfileByID(id)
		if err != nil {
			return 0, err
		}
		if profile != nil {
			beforeStatuses[id] = strings.TrimSpace(profile.Status)
		}
	}
	updated, err := s.repo.BatchUpdateProfileStatus(normalizedIDs, nextStatus, time.Now())
	if err != nil {
		return 0, err
	}
	for _, id := range normalizedIDs {
		before, ok := beforeStatuses[id]
		if !ok || before == nextStatus {
			continue
		}
		var targetUserID uint
		if profile, perr := s.repo.GetProfileByID(id); perr == nil && profile != nil {
			targetUserID = profile.UserID
		}
		s.recordAudit("affiliate_profile_status_changed", adminID, targetUserID, map[string]interface{}{
			"profile_id":    id,
			"before_status": before,
			"after_status":  nextStatus,
			"batch":         true,
		})
	}
	return updated, nil
}

// getOrCreateProfileInTx 在事务内获取或懒创建用户的 affiliate profile。
// 并发安全：user_id unique index 作为最终防线；duplicate key 后重新读取。
// 用于 commission 生成和 ApproveApplication，确保每个受益人都有 commission anchor。
func (s *Service) getOrCreateProfileInTx(tx affiliatecontract.Store, userID uint) (*affiliatedomain.Profile, error) {
	if tx == nil || userID == 0 {
		return nil, ErrNotFound
	}
	existing, err := tx.GetProfileByUserID(userID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
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
				// user_id 或 affiliate_code 冲突：重新读取（另一并发请求已创建）
				reloaded, reloadErr := tx.GetProfileByUserID(userID)
				if reloadErr != nil {
					return nil, reloadErr
				}
				if reloaded != nil {
					return reloaded, nil
				}
				continue // affiliate_code 冲突，重试生成
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

// OpenAffiliate 为用户开通推广返利（已退休，请使用 ApplyAffiliate）。
// 旧直接创建 active profile 的入口已关闭，现在需要用户提交申请 → 管理员审核 → 开通。
func (s *Service) OpenAffiliate(userID uint) (*affiliatedomain.Profile, error) {
	return nil, ErrOpenRetired
}

func normalizeAffiliateProfileIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return []uint{}
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func generateAffiliateCode() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var builder strings.Builder
	builder.Grow(affiliateCodeLength)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < affiliateCodeLength; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}
	return builder.String(), nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

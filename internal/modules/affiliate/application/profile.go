package application

import (
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	affiliatedomain "github.com/Aether-v1/hcz/internal/modules/affiliate/domain"

	"github.com/Aether-v1/hcz/internal/constants"
)

// UpdateAffiliateProfileStatus 管理端更新返利用户状态
func (s *Service) UpdateAffiliateProfileStatus(profileID uint, rawStatus string) (*affiliatedomain.Profile, error) {
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
	if strings.TrimSpace(profile.Status) == nextStatus {
		return profile, nil
	}
	if err := s.repo.UpdateProfileStatus(profileID, nextStatus, time.Now()); err != nil {
		return nil, err
	}
	return s.repo.GetProfileByID(profileID)
}

// BatchUpdateAffiliateProfileStatus 管理端批量更新返利用户状态
func (s *Service) BatchUpdateAffiliateProfileStatus(profileIDs []uint, rawStatus string) (int64, error) {
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
	return s.repo.BatchUpdateProfileStatus(normalizedIDs, nextStatus, time.Now())
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

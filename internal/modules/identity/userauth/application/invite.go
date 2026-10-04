package application

import (
	"errors"

	userdomain "github.com/Aether-v1/hcz/internal/modules/identity/user/domain"
)

// inviterFinder 按用户 ID 读取用户（用于向上遍历上级链）。
type inviterFinder func(uint) (*userdomain.User, error)

// IsDescendant 判断 candidate 是否位于 inviterID 的祖先链中。
//
// 即：从 inviterID 出发沿 inviter_id 向上遍历，若中途经过 candidate，
// 说明 inviterID 是 candidate 的下级——此时若把 candidate 挂到 inviterID 下会成环。
//
// finder 为 nil 或 inviterID 为 0 时返回 false。遍历有最大深度保护，避免脏数据死循环。
func IsDescendant(inviterID, candidate uint, finder inviterFinder) bool {
	if finder == nil || inviterID == 0 || candidate == 0 {
		return false
	}
	const maxDepth = 64 // 远超实际层级，防御脏数据导致的死循环
	current := inviterID
	for i := 0; i < maxDepth; i++ {
		if current == 0 {
			return false
		}
		if current == candidate {
			return true
		}
		parent, err := finder(current)
		if err != nil || parent == nil {
			return false
		}
		if parent.InviterID == nil || *parent.InviterID == 0 {
			return false
		}
		current = *parent.InviterID
	}
	return false
}

// CheckInviteBinding 校验把 newUserID 挂到 inviterID 下是否合法。
//   - inviterID == 0：无上级，合法
//   - inviterID == newUserID：自邀，拒绝
//   - newUserID 位于 inviterID 的祖先链中：成环，拒绝
//
// 注册时新用户尚未落库（newUserID=0），自邀/成环不会触发，此函数同时服务于未来 Admin 调整关系。
func CheckInviteBinding(inviterID, newUserID uint, finder inviterFinder) error {
	if inviterID == 0 {
		return nil
	}
	if inviterID == newUserID {
		return ErrSelfInvite
	}
	if IsDescendant(inviterID, newUserID, finder) {
		return ErrInviteCycle
	}
	return nil
}

// generateUniqueInviteCode 生成一个全局未占用的个人邀请码，碰撞则重试。
func (s *Service) generateUniqueInviteCode() (string, error) {
	const maxAttempts = 16
	for i := 0; i < maxAttempts; i++ {
		code, err := userdomain.GenerateInviteCode()
		if err != nil {
			return "", err
		}
		existing, err := s.userRepo.GetByInviteCode(code)
		if err != nil {
			return "", err
		}
		if existing == nil {
			return code, nil
		}
	}
	return "", errors.New("failed to allocate unique invite code after retries")
}

// resolveRegistrationInviter 解析注册时的直接上级用户 ID。
// 优先级：显式 invite_code > cookie/click 归因 > 无上级。
//   - 显式 invite_code 无效/不存在：返回 ErrInviteCodeInvalid（硬失败，阻止注册）
//   - cookie 归因失败或无归因：静默降级为无上级（返回 0, nil）
//
// 返回的 uint 为上级用户 ID；为 0 表示无上级。
func (s *Service) resolveRegistrationInviter(explicitInviteCode, visitorKey string) (uint, error) {
	code := userdomain.NormalizeInviteCode(explicitInviteCode)
	if code != "" {
		inviter, err := s.userRepo.GetByInviteCode(code)
		if err != nil {
			return 0, err
		}
		if inviter == nil {
			return 0, ErrInviteCodeInvalid
		}
		// 注册时新用户 ID=0，自邀/成环不会触发；此处为防御性校验（未来复用同一解析逻辑时生效）。
		if err := CheckInviteBinding(inviter.ID, 0, s.userRepo.GetByID); err != nil {
			return 0, err
		}
		return inviter.ID, nil
	}

	// 无显式邀请码：回退到 cookie/click 归因。任何异常都静默降级。
	if s.attributor != nil && visitorKey != "" {
		if inviterID, err := s.attributor.ResolveRegistrationInviterUserID(visitorKey); err == nil && inviterID > 0 {
			// 防御：归因到的上级必须真实存在且未被删除。
			if inviter, err := s.userRepo.GetByID(inviterID); err == nil && inviter != nil {
				return inviter.ID, nil
			}
		}
	}
	return 0, nil
}

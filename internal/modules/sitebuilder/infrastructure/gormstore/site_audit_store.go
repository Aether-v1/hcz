package gormstore

import (
	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	"gorm.io/gorm"
)

// SiteAuditStore 站点装修审计日志数据访问。
type SiteAuditStore struct {
	db *gorm.DB
}

// NewSiteAuditStore 创建 SiteAuditStore。
func NewSiteAuditStore(db *gorm.DB) *SiteAuditStore {
	return &SiteAuditStore{db: db}
}

// Create 写入审计日志。
func (s *SiteAuditStore) Create(log *sitebuilderdomain.SiteAuditLog) error {
	return s.db.Create(log).Error
}

// List 按时间倒序列出审计日志，支持分页。
func (s *SiteAuditStore) List(limit, offset int) ([]sitebuilderdomain.SiteAuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []sitebuilderdomain.SiteAuditLog
	if err := s.db.Order("id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

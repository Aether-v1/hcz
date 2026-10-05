package application

import (
	"encoding/json"

	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
)

// SiteAuditWriter 审计日志写入端口。
type SiteAuditWriter interface {
	Create(log *sitebuilderdomain.SiteAuditLog) error
}

// SiteAuditReader 审计日志读取端口。
type SiteAuditReader interface {
	List(limit, offset int) ([]sitebuilderdomain.SiteAuditLog, error)
}

// AuditService 站点装修审计日志服务。
type AuditService struct {
	writer SiteAuditWriter
	reader SiteAuditReader
}

// NewAuditService 创建 AuditService。
func NewAuditService(writer SiteAuditWriter, reader SiteAuditReader) *AuditService {
	return &AuditService{writer: writer, reader: reader}
}

// Record 写入一条审计日志。序列化失败时退化为字符串，不阻断主流程。
func (s *AuditService) Record(adminID uint, section, action string, before, after interface{}) {
	if s == nil || s.writer == nil {
		return
	}
	siteAuditLog := &sitebuilderdomain.SiteAuditLog{
		AdminID: adminID,
		Section: section,
		Action:  action,
		Before:  marshalAuditValue(before),
		After:   marshalAuditValue(after),
	}
	_ = s.writer.Create(siteAuditLog)
}

// List 列出审计日志。
func (s *AuditService) List(limit, offset int) ([]sitebuilderdomain.SiteAuditLog, error) {
	if s == nil || s.reader == nil {
		return []sitebuilderdomain.SiteAuditLog{}, nil
	}
	return s.reader.List(limit, offset)
}

func marshalAuditValue(v interface{}) string {
	if v == nil {
		return ""
	}
	if text, ok := v.(string); ok {
		return text
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(bytes)
}

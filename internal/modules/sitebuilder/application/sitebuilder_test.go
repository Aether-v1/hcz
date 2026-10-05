package application

import (
	"testing"

	sitebuilderdomain "github.com/Aether-v1/hcz/internal/modules/sitebuilder/domain"
	"github.com/Aether-v1/hcz/internal/modules/sitebuilder/infrastructure/gormstore"
	"github.com/Aether-v1/hcz/internal/shared/jsonmap"
)

// ==================== 校验函数 ====================

func TestValidateExternalURL(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"http ok", "https://example.com/path", false},
		{"http ok", "http://example.com", false},
		{"javascript rejected", "javascript:alert(1)", true},
		{"data rejected", "data:text/html,<h1>x</h1>", true},
		{"file rejected", "file:///etc/passwd", true},
		{"vbscript rejected", "vbscript:msgbox(1)", true},
		{"empty rejected", "", true},
		{"relative rejected", "/recharge", true},
		{"case-insensitive scheme", "JavaScript:alert(1)", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateExternalURL(tc.raw)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateExternalURL(%q) err=%v, wantErr=%v", tc.raw, err, tc.wantErr)
			}
		})
	}
}

func TestValidatePrimaryColor(t *testing.T) {
	valid := []string{"#4F46E5", "#fff", "#FFFFFF", "#000000"}
	for _, c := range valid {
		if err := ValidatePrimaryColor(c); err != nil {
			t.Fatalf("expected %s valid, got %v", c, err)
		}
	}
	invalid := []string{"4F46E5", "#GGG", "#12345", "red", ""}
	for _, c := range invalid {
		if err := ValidatePrimaryColor(c); err == nil {
			t.Fatalf("expected %s invalid", c)
		}
	}
}

func TestRouteWhitelist(t *testing.T) {
	allowed := []string{"recharge", "c2c", "wallet", "withdrawal", "invitation", "support", "orders"}
	for _, r := range allowed {
		if !IsAllowedHomeEntryRoute(r) {
			t.Fatalf("expected route %s allowed", r)
		}
	}
	denied := []string{"admin", "user", "settings", "", "rechargeX"}
	for _, r := range denied {
		if IsAllowedHomeEntryRoute(r) {
			t.Fatalf("expected route %s denied", r)
		}
	}
}

// ==================== Discovery Schema ====================

func TestParseDiscoveryConfigInvalidType(t *testing.T) {
	if _, err := ParseDiscoveryConfig("unknown_type", jsonmap.JSON{}); err == nil {
		t.Fatal("expected invalid type error")
	}
}

func TestParseDiscoveryConfigBanner(t *testing.T) {
	valid := jsonmap.JSON{"image": "https://cdn.example.com/a.png", "link_type": "none"}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeBanner, valid); err != nil {
		t.Fatalf("valid banner should pass: %v", err)
	}
	invalid := jsonmap.JSON{"image": ""}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeBanner, invalid); err == nil {
		t.Fatal("banner without image should fail")
	}
}

func TestParseDiscoveryConfigCardGrid(t *testing.T) {
	valid := jsonmap.JSON{
		"cards": []interface{}{
			map[string]interface{}{"title": "A", "action_type": "internal", "action_target": "recharge"},
		},
	}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeCardGrid, valid); err != nil {
		t.Fatalf("valid card_grid should pass: %v", err)
	}
	badAction := jsonmap.JSON{
		"cards": []interface{}{
			map[string]interface{}{"title": "A", "action_type": "external", "action_target": "javascript:alert(1)"},
		},
	}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeCardGrid, badAction); err == nil {
		t.Fatal("card with javascript url should fail")
	}
}

func TestParseDiscoveryConfigBusinessRecommend(t *testing.T) {
	valid := jsonmap.JSON{"product_ids": []interface{}{float64(1), float64(2)}}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeBusinessRecommend, valid); err != nil {
		t.Fatalf("valid business_recommend should pass: %v", err)
	}
	invalid := jsonmap.JSON{"product_ids": []interface{}{}}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeBusinessRecommend, invalid); err == nil {
		t.Fatal("empty product_ids should fail")
	}
}

func TestParseDiscoveryConfigAnnouncement(t *testing.T) {
	valid := jsonmap.JSON{"text": "公告内容", "link_type": "none"}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeAnnouncement, valid); err != nil {
		t.Fatalf("valid announcement should pass: %v", err)
	}
	invalid := jsonmap.JSON{"text": ""}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeAnnouncement, invalid); err == nil {
		t.Fatal("empty text should fail")
	}
}

func TestParseDiscoveryConfigExternalLink(t *testing.T) {
	valid := jsonmap.JSON{"label": "TG", "url": "https://t.me/foo"}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeExternalLink, valid); err != nil {
		t.Fatalf("valid external_link should pass: %v", err)
	}
	invalid := jsonmap.JSON{"label": "TG", "url": "javascript:alert(1)"}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeExternalLink, invalid); err == nil {
		t.Fatal("javascript url should fail")
	}
}

func TestParseDiscoveryConfigCategoryEntry(t *testing.T) {
	valid := jsonmap.JSON{"category_ids": []interface{}{float64(3)}}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeCategoryEntry, valid); err != nil {
		t.Fatalf("valid category_entry should pass: %v", err)
	}
	invalid := jsonmap.JSON{"category_ids": []interface{}{}}
	if _, err := ParseDiscoveryConfig(DiscoveryTypeCategoryEntry, invalid); err == nil {
		t.Fatal("empty category_ids should fail")
	}
}

// ==================== Home Entry Service (fake store) ====================

type fakeHomeEntryStore struct {
	entries map[uint]*sitebuilderdomain.HomeEntry
	nextID  uint
}

func newFakeHomeEntryStore() *fakeHomeEntryStore {
	return &fakeHomeEntryStore{entries: map[uint]*sitebuilderdomain.HomeEntry{}}
}

func (s *fakeHomeEntryStore) Create(e *sitebuilderdomain.HomeEntry) error {
	s.nextID++
	e.ID = s.nextID
	cp := *e
	s.entries[e.ID] = &cp
	return nil
}
func (s *fakeHomeEntryStore) Update(e *sitebuilderdomain.HomeEntry) error {
	cp := *e
	s.entries[e.ID] = &cp
	return nil
}
func (s *fakeHomeEntryStore) Delete(id uint) error { delete(s.entries, id); return nil }
func (s *fakeHomeEntryStore) GetByID(id uint) (*sitebuilderdomain.HomeEntry, error) {
	if e, ok := s.entries[id]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, nil
}
func (s *fakeHomeEntryStore) List(enabledOnly bool) ([]sitebuilderdomain.HomeEntry, error) {
	result := make([]sitebuilderdomain.HomeEntry, 0)
	for _, e := range s.entries {
		if enabledOnly && !e.Enabled {
			continue
		}
		cp := *e
		result = append(result, cp)
	}
	return result, nil
}
func (s *fakeHomeEntryStore) Reorder(items []gormstore.ReorderItem) error { return nil }

func TestHomeEntryServiceCreateInternalWhitelist(t *testing.T) {
	svc := NewHomeEntryService(newFakeHomeEntryStore())
	entry, err := svc.Create(HomeEntryInput{
		Title: "充值", ActionType: "internal", ActionTarget: "recharge", Icon: "recharge",
	})
	if err != nil {
		t.Fatalf("expected valid internal entry, got %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("expected ID assigned")
	}
}

func TestHomeEntryServiceCreateInternalDeniedRoute(t *testing.T) {
	svc := NewHomeEntryService(newFakeHomeEntryStore())
	_, err := svc.Create(HomeEntryInput{
		Title: "恶意", ActionType: "internal", ActionTarget: "admin/settings",
	})
	if err == nil {
		t.Fatal("expected non-whitelist route to be rejected")
	}
}

func TestHomeEntryServiceCreateExternalRejectsJavascript(t *testing.T) {
	svc := NewHomeEntryService(newFakeHomeEntryStore())
	_, err := svc.Create(HomeEntryInput{
		Title: "外链", ActionType: "external", ActionTarget: "javascript:alert(1)",
	})
	if err == nil {
		t.Fatal("expected javascript url to be rejected")
	}
}

func TestHomeEntryServiceUpdateAndToggle(t *testing.T) {
	store := newFakeHomeEntryStore()
	svc := NewHomeEntryService(store)
	entry, _ := svc.Create(HomeEntryInput{Title: "充值", ActionType: "internal", ActionTarget: "recharge"})
	updated, err := svc.Update(entry.ID, HomeEntryInput{Title: "充值2", ActionType: "internal", ActionTarget: "wallet"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.ActionTarget != "wallet" {
		t.Fatalf("expected action_target updated, got %s", updated.ActionTarget)
	}
	disabled, err := svc.SetEnabled(entry.ID, false)
	if err != nil {
		t.Fatalf("toggle failed: %v", err)
	}
	if disabled.Enabled {
		t.Fatal("expected disabled")
	}
	list, _ := svc.ListPublic()
	if len(list) != 0 {
		t.Fatalf("expected no public entries after disable, got %d", len(list))
	}
}

// ==================== Audit Service ====================

type fakeAuditWriter struct {
	logs []*sitebuilderdomain.SiteAuditLog
}

func (w *fakeAuditWriter) Create(log *sitebuilderdomain.SiteAuditLog) error {
	w.logs = append(w.logs, log)
	return nil
}

func (w *fakeAuditWriter) List(limit, offset int) ([]sitebuilderdomain.SiteAuditLog, error) {
	result := make([]sitebuilderdomain.SiteAuditLog, 0, len(w.logs))
	for i := len(w.logs) - 1; i >= 0; i-- {
		result = append(result, *w.logs[i])
	}
	return result, nil
}

func TestAuditServiceRecord(t *testing.T) {
	w := &fakeAuditWriter{}
	svc := NewAuditService(w, w)
	svc.Record(42, "home_entries", "create", nil, map[string]string{"title": "充值"})
	if len(w.logs) != 1 {
		t.Fatalf("expected 1 audit log, got %d", len(w.logs))
	}
	if w.logs[0].AdminID != 42 || w.logs[0].Section != "home_entries" || w.logs[0].Action != "create" {
		t.Fatalf("unexpected audit entry: %+v", w.logs[0])
	}
	logs, err := svc.List(50, 0)
	if err != nil || len(logs) != 1 {
		t.Fatalf("expected list 1, got %d err %v", len(logs), err)
	}
}

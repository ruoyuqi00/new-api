package model

import (
	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type QualityMonitorQuestion struct {
	ID             string `json:"id"`
	Prompt         string `json:"prompt"`
	ExpectedAnswer string `json:"expected_answer"`
	MatchType      string `json:"match_type"`
}

// Five maximum-size prompts and reference answers exceed MySQL TEXT capacity.
// Keep JSON as text while selecting sufficient capacity for that dialect.
type QualityMonitorQuestionsJSON string

func (QualityMonitorQuestionsJSON) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db != nil && db.Dialector.Name() == "mysql" {
		return "LONGTEXT"
	}
	return "TEXT"
}

type QualityMonitorPlan struct {
	ID              int                         `json:"id" gorm:"primaryKey"`
	Name            string                      `json:"name" gorm:"type:varchar(128)"`
	Enabled         bool                        `json:"enabled" gorm:"index"`
	Published       bool                        `json:"published" gorm:"index"`
	Groups          []string                    `json:"groups" gorm:"-"`
	Models          []string                    `json:"models" gorm:"-"`
	Questions       []QualityMonitorQuestion    `json:"questions" gorm:"-"`
	GroupsJSON      string                      `json:"-" gorm:"type:text"`
	ModelsJSON      string                      `json:"-" gorm:"type:text"`
	QuestionsJSON   QualityMonitorQuestionsJSON `json:"-"`
	IntervalMinutes int                         `json:"interval_minutes"`
	ReasoningEffort string                      `json:"reasoning_effort" gorm:"type:varchar(16)"`
	MaxOutputTokens int                         `json:"max_output_tokens"`
	TimeoutSeconds  int                         `json:"timeout_seconds"`
	LastRunAt       int64                       `json:"last_run_at"`
	NextRunAt       int64                       `json:"next_run_at" gorm:"bigint;index"`
	CreatedAt       int64                       `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64                       `json:"updated_at" gorm:"bigint"`
}

func (p *QualityMonitorPlan) BeforeSave(_ *gorm.DB) error {
	groups, err := common.Marshal(p.Groups)
	if err != nil {
		return err
	}
	models, err := common.Marshal(p.Models)
	if err != nil {
		return err
	}
	questions, err := common.Marshal(p.Questions)
	if err != nil {
		return err
	}
	p.GroupsJSON = string(groups)
	p.ModelsJSON = string(models)
	p.QuestionsJSON = QualityMonitorQuestionsJSON(questions)
	return nil
}

func (p *QualityMonitorPlan) AfterFind(_ *gorm.DB) error {
	if err := common.UnmarshalJsonStr(p.GroupsJSON, &p.Groups); err != nil {
		return err
	}
	if err := common.UnmarshalJsonStr(p.ModelsJSON, &p.Models); err != nil {
		return err
	}
	return common.UnmarshalJsonStr(string(p.QuestionsJSON), &p.Questions)
}

func SaveQualityMonitorPlan(p *QualityMonitorPlan, create bool) error {
	now := common.GetTimestamp()
	p.UpdatedAt = now
	if create {
		p.ID = 0
		p.Enabled = false
		p.Published = false
		p.CreatedAt = now
		p.LastRunAt = 0
		p.NextRunAt = 0
		return DB.Create(p).Error
	}
	existing, err := GetQualityMonitorPlan(p.ID)
	if err != nil {
		return err
	}
	if err := p.BeforeSave(nil); err != nil {
		return err
	}
	updates := map[string]any{"name": p.Name, "enabled": p.Enabled, "published": p.Published, "groups_json": p.GroupsJSON, "models_json": p.ModelsJSON, "questions_json": p.QuestionsJSON, "interval_minutes": p.IntervalMinutes, "reasoning_effort": p.ReasoningEffort, "max_output_tokens": p.MaxOutputTokens, "timeout_seconds": p.TimeoutSeconds, "updated_at": now}
	if !p.Enabled && existing.Enabled {
		updates["next_run_at"] = 0
	} else if p.Enabled && !existing.Enabled {
		updates["next_run_at"] = gorm.Expr("CASE WHEN next_run_at > ? THEN next_run_at ELSE ? END", now, now)
	}
	// Update config columns only: scheduling belongs to the worker, and Save's
	// insert fallback would otherwise recreate a concurrently deleted plan.
	if err := DB.Model(&QualityMonitorPlan{}).Where("id = ?", p.ID).Updates(updates).Error; err != nil {
		return err
	}
	current, err := GetQualityMonitorPlan(p.ID)
	if err == nil {
		*p = *current
	}
	return err
}

func GetQualityMonitorPlan(id int) (*QualityMonitorPlan, error) {
	var p QualityMonitorPlan
	err := DB.First(&p, id).Error
	return &p, err
}

func ListQualityMonitorPlans() ([]QualityMonitorPlan, error) {
	plans := make([]QualityMonitorPlan, 0)
	err := DB.Order("id desc").Find(&plans).Error
	return plans, err
}

func DeleteQualityMonitorPlan(id int) error { return DB.Delete(&QualityMonitorPlan{}, id).Error }

type QualityMonitorResult struct {
	ID                  int    `json:"id" gorm:"primaryKey"`
	PlanID              int    `json:"plan_id" gorm:"index"`
	PlanName            string `json:"plan_name" gorm:"type:varchar(128)"`
	Group               string `json:"group" gorm:"type:varchar(64);index"`
	Model               string `json:"model" gorm:"type:varchar(255);index"`
	QuestionID          string `json:"question_id" gorm:"type:varchar(64)"`
	Prompt              string `json:"prompt" gorm:"type:text"`
	ExpectedAnswer      string `json:"expected_answer" gorm:"type:text"`
	Answer              string `json:"answer" gorm:"type:text"`
	Status              string `json:"status" gorm:"type:varchar(16)"`
	DurationMS          int64  `json:"duration_ms"`
	CreatedAt           int64  `json:"created_at" gorm:"bigint;index"`
	AnswerTruncated     bool   `json:"answer_truncated"`
	ReasoningEffort     string `json:"reasoning_effort" gorm:"type:varchar(16)"`
	ChannelID           int    `json:"channel_id,omitempty"`
	Error               string `json:"error,omitempty" gorm:"type:text"`
	ActualResponseModel string `json:"actual_response_model,omitempty" gorm:"type:varchar(255)"`
	EstimatedQuota      int    `json:"estimated_quota,omitempty"`
}

type QualityMonitorResultFilter struct {
	Admin         bool
	AllowedGroups []string
	Group         string
	Model         string
	PlanID        int
	Page          int
	PageSize      int
}

func ListQualityMonitorResults(f QualityMonitorResultFilter) ([]QualityMonitorResult, int64, error) {
	items := make([]QualityMonitorResult, 0)
	query := DB.Model(&QualityMonitorResult{}).Where("created_at >= ?", common.GetTimestamp()-30*86400)
	if !f.Admin {
		if len(f.AllowedGroups) == 0 {
			return items, 0, nil
		}
		query = query.Where(commonGroupCol+" IN ?", f.AllowedGroups).Where("plan_id IN (?)", DB.Model(&QualityMonitorPlan{}).Select("id").Where("published = ?", true))
	}
	if f.Group != "" {
		query = query.Where(commonGroupCol+" = ?", f.Group)
	}
	if f.Model != "" {
		query = query.Where("model = ?", f.Model)
	}
	if f.PlanID > 0 {
		query = query.Where("plan_id = ?", f.PlanID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 {
		f.PageSize = 20
	}
	if f.PageSize > 100 {
		f.PageSize = 100
	}
	err := query.Order("id desc").Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&items).Error
	return items, total, err
}

func CleanupQualityMonitorResults(now int64) (int64, error) {
	var ids []int
	if err := DB.Model(&QualityMonitorResult{}).Where("created_at < ?", now-30*86400).Order("id asc").Limit(500).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Where("id IN ?", ids).Delete(&QualityMonitorResult{})
	return result.RowsAffected, result.Error
}

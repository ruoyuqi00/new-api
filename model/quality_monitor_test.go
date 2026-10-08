package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Removing TEXT config serialization or using zero-value-skipping updates breaks
// these stored-plan and publication contracts.
func TestQualityMonitorPlanRoundTripAndPausedDefaults(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&QualityMonitorPlan{}, &QualityMonitorResult{}))
	t.Cleanup(func() {
		DB.Where("1 = 1").Delete(&QualityMonitorResult{})
		DB.Where("1 = 1").Delete(&QualityMonitorPlan{})
	})
	plan := QualityMonitorPlan{Name: "Routing check", Groups: []string{"alpha", "beta"}, Models: []string{"gpt-6-astra"}, IntervalMinutes: 10, ReasoningEffort: "high", MaxOutputTokens: 512, TimeoutSeconds: 30, Questions: []QualityMonitorQuestion{{ID: "q1", Prompt: "What is six times seven?", ExpectedAnswer: "42", MatchType: "exact"}}}
	require.NoError(t, SaveQualityMonitorPlan(&plan, true))
	assert.False(t, plan.Enabled)
	assert.False(t, plan.Published)
	assert.Zero(t, plan.NextRunAt)
	stored, err := GetQualityMonitorPlan(plan.ID)
	require.NoError(t, err)
	assert.Equal(t, plan.Groups, stored.Groups)
	assert.Equal(t, plan.Questions, stored.Questions)
	stored.Enabled = true
	stored.Published = true
	require.NoError(t, SaveQualityMonitorPlan(stored, false))
	assert.GreaterOrEqual(t, stored.NextRunAt, common.GetTimestamp())
	stored.Enabled = false
	stored.Published = false
	require.NoError(t, SaveQualityMonitorPlan(stored, false))
	reloaded, err := GetQualityMonitorPlan(plan.ID)
	require.NoError(t, err)
	assert.False(t, reloaded.Enabled)
	assert.False(t, reloaded.Published)
	assert.Zero(t, reloaded.NextRunAt)
}

func TestQualityMonitorLargeQuestionSetUsesPortableTextStorage(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&QualityMonitorPlan{}))
	t.Cleanup(func() { DB.Where("1 = 1").Delete(&QualityMonitorPlan{}) })
	plan := QualityMonitorPlan{Name: "large", Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}}
	for i := 0; i < 5; i++ {
		plan.Questions = append(plan.Questions, QualityMonitorQuestion{ID: fmt.Sprint(i), Prompt: strings.Repeat("p", 8192), ExpectedAnswer: strings.Repeat("e", 8192), MatchType: "exact"})
	}
	require.NoError(t, SaveQualityMonitorPlan(&plan, true))
	stored, err := GetQualityMonitorPlan(plan.ID)
	require.NoError(t, err)
	assert.Equal(t, plan.Questions, stored.Questions)
	dialects := []struct {
		name      string
		dialector gorm.Dialector
		want      string
	}{
		{"mysql", mysql.New(mysql.Config{DSN: "local:local@tcp(127.0.0.1:1)/local", SkipInitializeWithVersion: true}), "LONGTEXT"},
		{"postgres", postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 user=local dbname=local sslmode=disable"}), "TEXT"},
	}
	for _, dialect := range dialects {
		t.Run(dialect.name, func(t *testing.T) {
			db, err := gorm.Open(dialect.dialector, &gorm.Config{DisableAutomaticPing: true, DryRun: true})
			require.NoError(t, err)
			stmt := &gorm.Statement{DB: db}
			require.NoError(t, stmt.Parse(&QualityMonitorPlan{}))
			field := stmt.Schema.LookUpField("QuestionsJSON")
			require.NotNil(t, field)
			assert.Equal(t, dialect.want, db.Migrator().FullDataTypeOf(field).SQL)
			// The manual-run snapshot travels through the existing task table,
			// whose payload needs the same capacity as the persisted questions.
			require.NoError(t, stmt.Parse(&SystemTask{}))
			field = stmt.Schema.LookUpField("Payload")
			require.NotNil(t, field)
			assert.Equal(t, dialect.want, db.Migrator().FullDataTypeOf(field).SQL)
		})
	}
}

func TestQualityMonitorConfigEditPreservesConcurrentReservation(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&QualityMonitorPlan{}))
	t.Cleanup(func() { DB.Where("1 = 1").Delete(&QualityMonitorPlan{}) })
	plan := QualityMonitorPlan{Name: "Before", Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, Questions: []QualityMonitorQuestion{}, IntervalMinutes: 5}
	require.NoError(t, SaveQualityMonitorPlan(&plan, true))
	plan.Enabled = true
	require.NoError(t, SaveQualityMonitorPlan(&plan, false))
	reserved := common.GetTimestamp() + 600
	injected := false
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register("quality_monitor_reservation_test", func(tx *gorm.DB) {
		if injected || tx.Statement.Table != "quality_monitor_plans" {
			return
		}
		injected = true
		require.NoError(t, tx.Exec("UPDATE quality_monitor_plans SET last_run_at = ?, next_run_at = ? WHERE id = ?", 123, reserved, plan.ID).Error)
	}))
	defer DB.Callback().Update().Remove("quality_monitor_reservation_test")
	plan.Name = "Edited"
	require.NoError(t, SaveQualityMonitorPlan(&plan, false))
	stored, err := GetQualityMonitorPlan(plan.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 123, stored.LastRunAt)
	assert.Equal(t, reserved, stored.NextRunAt)
	assert.Equal(t, "Edited", stored.Name)
}

func TestQualityMonitorConcurrentDeleteDoesNotRecreatePlan(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&QualityMonitorPlan{}))
	plan := QualityMonitorPlan{Name: "Before", Groups: []string{}, Models: []string{}, Questions: []QualityMonitorQuestion{}}
	require.NoError(t, SaveQualityMonitorPlan(&plan, true))
	injected := false
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register("quality_monitor_delete_test", func(tx *gorm.DB) {
		if injected || tx.Statement.Table != "quality_monitor_plans" {
			return
		}
		injected = true
		require.NoError(t, tx.Exec("DELETE FROM quality_monitor_plans WHERE id = ?", plan.ID).Error)
	}))
	defer DB.Callback().Update().Remove("quality_monitor_delete_test")
	require.Error(t, SaveQualityMonitorPlan(&plan, false))
	var count int64
	require.NoError(t, DB.Model(&QualityMonitorPlan{}).Where("id = ?", plan.ID).Count(&count).Error)
	assert.Zero(t, count)
}

// Expired retention must never delete a current answer, another table, or more
// than one bounded batch, and deleted plans must retain their history.
func TestQualityMonitorResultsPermissionsRetentionAndPlanDeletion(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&QualityMonitorPlan{}, &QualityMonitorResult{}))
	t.Cleanup(func() {
		DB.Where("1 = 1").Delete(&QualityMonitorResult{})
		DB.Where("1 = 1").Delete(&QualityMonitorPlan{})
	})
	published := QualityMonitorPlan{Name: "Published", Published: true, Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, Questions: []QualityMonitorQuestion{}}
	private := QualityMonitorPlan{Name: "Private", Groups: []string{"alpha"}, Models: []string{"gpt-6-astra"}, Questions: []QualityMonitorQuestion{}}
	require.NoError(t, DB.Create(&published).Error)
	require.NoError(t, DB.Create(&private).Error)
	now := common.GetTimestamp()
	rows := []QualityMonitorResult{{PlanID: published.ID, Group: "alpha", Model: "gpt-6-astra", Answer: "42", CreatedAt: now}, {PlanID: published.ID, Group: "beta", Model: "gpt-6-astra", Answer: "secret-group", CreatedAt: now}, {PlanID: private.ID, Group: "alpha", Model: "gpt-6-astra", Answer: "private", CreatedAt: now}}
	require.NoError(t, DB.Create(&rows).Error)
	items, total, err := ListQualityMonitorResults(QualityMonitorResultFilter{Page: 1, PageSize: 20, AllowedGroups: []string{"alpha"}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	assert.Equal(t, "42", items[0].Answer)
	for i := 0; i < 501; i++ {
		require.NoError(t, DB.Create(&QualityMonitorResult{PlanID: published.ID, Answer: fmt.Sprint(i), CreatedAt: now - 31*86400}).Error)
	}
	deleted, err := CleanupQualityMonitorResults(now)
	require.NoError(t, err)
	assert.EqualValues(t, 500, deleted)
	require.NoError(t, DeleteQualityMonitorPlan(published.ID))
	_, total, err = ListQualityMonitorResults(QualityMonitorResultFilter{Admin: true, Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	var remaining int64
	require.NoError(t, DB.Model(&QualityMonitorResult{}).Count(&remaining).Error)
	assert.EqualValues(t, 4, remaining) // expired tail remains until the next batch
}

package repository

import (
	"path/filepath"
	"testing"
	"time"

	"gold_price/backend/internal/model"
)

func TestFactorRepositoryUpsertAndQuerySnapshots(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "factor-repository.db")
	db, err := model.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	repo := NewFactorRepository(db)
	if err := repo.UpsertDefinitions([]model.FactorDefinitionRecord{
		{
			Code:                "usd_index",
			Name:                "美元指数",
			Category:            "macro",
			Description:         "美元走强通常压制黄金表现。",
			ValueType:           "number",
			DefaultWeight:       0.96,
			ImpactDirectionRule: "value 上行通常利空黄金",
		},
	}); err != nil {
		t.Fatalf("upsert definitions: %v", err)
	}

	definition, err := repo.GetDefinitionByCode("usd_index")
	if err != nil {
		t.Fatalf("get definition: %v", err)
	}

	firstTime := time.Now().Add(-time.Hour)
	secondTime := time.Now()
	if err := repo.SaveSnapshots([]model.FactorSnapshotRecord{
		{
			FactorID:        definition.ID,
			SourceID:        1,
			ValueNum:        103.2,
			Score:           -54.5,
			ImpactDirection: "bearish",
			ImpactStrength:  63,
			Confidence:      82,
			Summary:         "美元指数偏强",
			CapturedAt:      firstTime,
		},
		{
			FactorID:        definition.ID,
			SourceID:        1,
			ValueNum:        104.1,
			Score:           -61.3,
			ImpactDirection: "bearish",
			ImpactStrength:  71,
			Confidence:      85,
			Summary:         "美元指数继续走强",
			CapturedAt:      secondTime,
		},
	}); err != nil {
		t.Fatalf("save snapshots: %v", err)
	}

	latest, err := repo.GetLatestSnapshotByFactorID(definition.ID)
	if err != nil {
		t.Fatalf("get latest snapshot: %v", err)
	}
	if latest.ValueNum != 104.1 {
		t.Fatalf("expected latest snapshot to be returned, got %.3f", latest.ValueNum)
	}

	records, err := repo.ListSnapshotsByFactorID(definition.ID, firstTime.Add(-time.Minute), secondTime.Add(time.Minute))
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(records))
	}
}

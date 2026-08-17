package importplan

import (
	"testing"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/codex"
	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

func TestBuildClassifiesMatchingAndMissingProjects(t *testing.T) {
	scan := codex.SyntheticScan()
	conversations := append([]domain.ConversationSummary{}, scan.Projects[0].Conversations...)
	conversations = append(conversations, scan.Projects[1].Conversations...)
	plan := Build("/tmp/example.zip", conversations, []domain.TargetProject{{ID: "target-market", Name: "市场调研", Directory: "/synthetic/projects/market-research"}})
	if len(plan.Items) != 2 || !plan.RequiresUserChoice {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Items[0].Status != "path-matched" {
		t.Fatalf("expected path match: %+v", plan.Items[0])
	}
	if plan.Items[1].Status != "no-target" || plan.Items[1].Action != "create-empty-project" {
		t.Fatalf("expected empty project action: %+v", plan.Items[1])
	}
}

func TestBuildRequiresChoiceForSameNameDifferentPath(t *testing.T) {
	scan := codex.SyntheticScan()
	plan := Build("/tmp/example.zip", scan.Projects[0].Conversations, []domain.TargetProject{{ID: "target", Name: "市场调研", Directory: "/Users/example/market-research"}})
	if len(plan.Items) != 1 || plan.Items[0].Status != "path-differs" || !plan.RequiresUserChoice {
		t.Fatalf("unexpected plan: %+v", plan)
	}
}

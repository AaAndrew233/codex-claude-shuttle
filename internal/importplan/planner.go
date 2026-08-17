package importplan

import (
	"path/filepath"

	"github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"
)

// Build 只生成导入计划，不创建目录、不写文件，也不触碰 Codex 私有索引。
func Build(bundlePath string, conversations []domain.ConversationSummary, targets []domain.TargetProject) domain.ImportPlan {
	byDirectory := make(map[string]domain.TargetProject, len(targets))
	for _, target := range targets {
		byDirectory[filepath.Clean(target.Directory)] = target
	}
	grouped := make(map[string][]domain.ConversationSummary)
	order := make([]string, 0)
	for _, conversation := range conversations {
		if _, exists := grouped[conversation.SourceProject]; !exists {
			order = append(order, conversation.SourceProject)
		}
		grouped[conversation.SourceProject] = append(grouped[conversation.SourceProject], conversation)
	}
	plan := domain.ImportPlan{BundlePath: bundlePath, Tool: "codex", Items: make([]domain.ImportPlanItem, 0, len(order)), ConversationCount: len(conversations)}
	for _, sourceProject := range order {
		items := grouped[sourceProject]
		sourceDirectory := filepath.Clean(items[0].SourceDirectory)
		if target, ok := byDirectory[sourceDirectory]; ok {
			plan.Items = append(plan.Items, domain.ImportPlanItem{SourceProject: sourceProject, TargetProject: target.Name, ConversationCount: len(items), Status: "path-matched", Action: "import", Reason: "来源目录与目标项目一致"})
			continue
		}
		nameMatches := make([]domain.TargetProject, 0, 1)
		for _, target := range targets {
			if target.Name == sourceProject {
				nameMatches = append(nameMatches, target)
			}
		}
		if len(nameMatches) == 1 {
			plan.RequiresUserChoice = true
			plan.Items = append(plan.Items, domain.ImportPlanItem{SourceProject: sourceProject, TargetProject: nameMatches[0].Name, ConversationCount: len(items), Status: "path-differs", Action: "choose-target", Reason: "项目名称相同但来源目录不同，需要用户确认"})
			continue
		}
		plan.RequiresUserChoice = true
		plan.Items = append(plan.Items, domain.ImportPlanItem{SourceProject: sourceProject, ConversationCount: len(items), Status: "no-target", Action: "create-empty-project", Reason: "没有对应目标项目，需要选择父目录并确认项目名称"})
	}
	if plan.RequiresUserChoice {
		plan.Message = "导入计划需要用户确认目标项目"
	} else {
		plan.Message = "所有项目均已匹配，可以进入预检"
	}
	return plan
}

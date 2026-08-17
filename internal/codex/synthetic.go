package codex

import "github.com/giraffegzy-bot/codex-claude-shuttle/internal/domain"

// SyntheticScan 返回仅用于开发闭环的合成夹具，绝不读取真实 Codex 目录。
func SyntheticScan() domain.ScanResult {
	return domain.ScanResult{
		Source: "synthetic-fixture",
		Projects: []domain.ProjectSummary{
			{
				ID:                "project-research",
				Name:              "市场调研",
				ConversationCount: 2,
				Conversations: []domain.ConversationSummary{
					{
						ID:              "conv-market-brief",
						Title:           "整理竞品调研结论",
						UpdatedAt:       "2026-08-12T14:20:00+08:00",
						UserMessage:     "请把今天的竞品调研整理成三条结论。",
						FinalReply:      "已整理为三条结论，并标注了待验证假设。",
						SourceProject:   "市场调研",
						SourceDirectory: "/synthetic/projects/market-research",
					},
					{
						ID:              "conv-market-next",
						Title:           "规划下一轮访谈",
						UpdatedAt:       "2026-08-11T09:05:00+08:00",
						UserMessage:     "根据现有发现，下一轮访谈先验证什么？",
						FinalReply:      "建议优先验证用户留存和迁移成本两个问题。",
						SourceProject:   "市场调研",
						SourceDirectory: "/synthetic/projects/market-research",
					},
				},
			},
			{
				ID:                "project-product",
				Name:              "产品设计",
				ConversationCount: 1,
				Conversations: []domain.ConversationSummary{
					{
						ID:              "conv-product-flow",
						Title:           "梳理导入冲突流程",
						UpdatedAt:       "2026-08-10T18:40:00+08:00",
						UserMessage:     "把没有对应项目时的导入流程讲清楚。",
						FinalReply:      "没有对应项目时建议创建空项目目录，再由用户确认后导入。",
						SourceProject:   "产品设计",
						SourceDirectory: "/synthetic/projects/product-design",
					},
				},
			},
		},
	}
}

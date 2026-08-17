package domain

// ProjectSummary 是用户可见的项目级摘要，不暴露完整路径。
type ProjectSummary struct {
	ID                string                `json:"id"`
	Name              string                `json:"name"`
	ConversationCount int                   `json:"conversationCount"`
	Conversations     []ConversationSummary `json:"conversations"`
}

// ConversationSummary 是安全的对话可见投影。
type ConversationSummary struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	UpdatedAt       string `json:"updatedAt"`
	UserMessage     string `json:"userMessage"`
	FinalReply      string `json:"finalReply"`
	SourceProject   string `json:"sourceProject"`
	SourceDirectory string `json:"sourceDirectory"`
}

type ScanResult struct {
	Source       string           `json:"source"`
	Projects     []ProjectSummary `json:"projects"`
	Partial      bool             `json:"partial"`
	SkippedFiles int              `json:"skippedFiles"`
	ScannedBytes int64            `json:"scannedBytes"`
	Message      string           `json:"message"`
}

type ExportRequest struct {
	Destination     string   `json:"destination"`
	ConversationIDs []string `json:"conversationIds"`
}

type ExportResult struct {
	Path              string `json:"path"`
	ProjectCount      int    `json:"projectCount"`
	ConversationCount int    `json:"conversationCount"`
	Bytes             int64  `json:"bytes"`
	IntegrityValid    bool   `json:"integrityValid"`
}

type BundleValidation struct {
	Valid             bool   `json:"valid"`
	Path              string `json:"path"`
	SourceTool        string `json:"sourceTool"`
	ProjectCount      int    `json:"projectCount"`
	ConversationCount int    `json:"conversationCount"`
	Bytes             int64  `json:"bytes"`
	Message           string `json:"message"`
}

// TransferImportSource 是迁移包中的来源项目摘要，不向前端暴露完整来源路径。
type TransferImportSource struct {
	Key                 string `json:"key"`
	Name                string `json:"name"`
	SourceFolder        string `json:"sourceFolder"`
	ConversationCount   int    `json:"conversationCount"`
	SuggestedTargetID   string `json:"suggestedTargetId"`
	SuggestedTargetName string `json:"suggestedTargetName"`
}

type TransferImportTarget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
}

type TransferImportInspection struct {
	BundlePath        string                 `json:"bundlePath"`
	SourceTool        string                 `json:"sourceTool"`
	ProjectCount      int                    `json:"projectCount"`
	ConversationCount int                    `json:"conversationCount"`
	Sources           []TransferImportSource `json:"sources"`
	Targets           []TransferImportTarget `json:"targets"`
	Message           string                 `json:"message"`
}

type TransferProjectMapping struct {
	SourceKey       string `json:"sourceKey"`
	TargetID        string `json:"targetId"`
	TargetDirectory string `json:"targetDirectory"`
}

type TransferImportRequest struct {
	BundlePath     string                   `json:"bundlePath"`
	SourceTool     string                   `json:"sourceTool"`
	TargetTool     string                   `json:"targetTool"`
	TargetRoot     string                   `json:"targetRoot"`
	Mappings       []TransferProjectMapping `json:"mappings"`
	ConflictPolicy string                   `json:"conflictPolicy"`
}

type TransferImportPreflight struct {
	PlanToken         string `json:"planToken"`
	SourceTool        string `json:"sourceTool"`
	TargetTool        string `json:"targetTool"`
	RequiredBytes     int64  `json:"requiredBytes"`
	AvailableBytes    uint64 `json:"availableBytes"`
	ProjectCount      int    `json:"projectCount"`
	ConversationCount int    `json:"conversationCount"`
	CreateCount       int    `json:"createCount"`
	SkipCount         int    `json:"skipCount"`
	ReplaceCount      int    `json:"replaceCount"`
	ToolRunning       bool   `json:"toolRunning"`
	CanExecute        bool   `json:"canExecute"`
	Message           string `json:"message"`
}

type TransferImportExecutionResult struct {
	SourceTool         string   `json:"sourceTool"`
	TargetTool         string   `json:"targetTool"`
	Written            int      `json:"written"`
	Skipped            int      `json:"skipped"`
	Replaced           int      `json:"replaced"`
	Recovered          bool     `json:"recovered"`
	FilesVerified      bool     `json:"filesVerified"`
	DiscoveryAttempted bool     `json:"discoveryAttempted"`
	Discovered         int      `json:"discovered"`
	PendingDiscovery   int      `json:"pendingDiscovery"`
	Warnings           []string `json:"warnings"`
	Message            string   `json:"message"`
}

type TargetProject struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Directory string `json:"directory"`
}

type ImportPlanItem struct {
	SourceProject     string `json:"sourceProject"`
	TargetProject     string `json:"targetProject"`
	ConversationCount int    `json:"conversationCount"`
	Status            string `json:"status"`
	Action            string `json:"action"`
	Reason            string `json:"reason"`
}

type ImportPlan struct {
	BundlePath         string           `json:"bundlePath"`
	Tool               string           `json:"tool"`
	Items              []ImportPlanItem `json:"items"`
	ConversationCount  int              `json:"conversationCount"`
	RequiresUserChoice bool             `json:"requiresUserChoice"`
	Message            string           `json:"message"`
}

type ImportPreflight struct {
	TargetRoot        string `json:"targetRoot"`
	RequiredBytes     int64  `json:"requiredBytes"`
	AvailableBytes    uint64 `json:"availableBytes"`
	Writable          bool   `json:"writable"`
	CodexClosed       bool   `json:"codexClosed"`
	ProjectCount      int    `json:"projectCount"`
	ConversationCount int    `json:"conversationCount"`
	CanExecute        bool   `json:"canExecute"`
	Message           string `json:"message"`
}

type ImportExecutionResult struct {
	Written   int    `json:"written"`
	Skipped   int    `json:"skipped"`
	Replaced  int    `json:"replaced"`
	Recovered bool   `json:"recovered"`
	Message   string `json:"message"`
}

type CodexImportSource struct {
	Key                 string `json:"key"`
	Name                string `json:"name"`
	SourceFolder        string `json:"sourceFolder"`
	ConversationCount   int    `json:"conversationCount"`
	SuggestedTargetID   string `json:"suggestedTargetId"`
	SuggestedTargetName string `json:"suggestedTargetName"`
}

type CodexImportTarget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Folder string `json:"folder"`
}

type CodexImportInspection struct {
	BundlePath        string              `json:"bundlePath"`
	ProjectCount      int                 `json:"projectCount"`
	ConversationCount int                 `json:"conversationCount"`
	Sources           []CodexImportSource `json:"sources"`
	Targets           []CodexImportTarget `json:"targets"`
	Message           string              `json:"message"`
}

type CodexProjectMapping struct {
	SourceKey       string `json:"sourceKey"`
	TargetID        string `json:"targetId"`
	TargetDirectory string `json:"targetDirectory"`
}

type CodexImportRequest struct {
	BundlePath     string                `json:"bundlePath"`
	CodexRoot      string                `json:"codexRoot"`
	Mappings       []CodexProjectMapping `json:"mappings"`
	ConflictPolicy string                `json:"conflictPolicy"`
}

type CodexImportPreflight struct {
	PlanToken         string `json:"planToken"`
	RequiredBytes     int64  `json:"requiredBytes"`
	AvailableBytes    uint64 `json:"availableBytes"`
	ProjectCount      int    `json:"projectCount"`
	ConversationCount int    `json:"conversationCount"`
	CreateCount       int    `json:"createCount"`
	SkipCount         int    `json:"skipCount"`
	ReplaceCount      int    `json:"replaceCount"`
	CodexRunning      bool   `json:"codexRunning"`
	CanExecute        bool   `json:"canExecute"`
	Message           string `json:"message"`
}

type CodexImportExecutionResult struct {
	Written            int      `json:"written"`
	Skipped            int      `json:"skipped"`
	Replaced           int      `json:"replaced"`
	Recovered          bool     `json:"recovered"`
	FilesVerified      bool     `json:"filesVerified"`
	DiscoveryAttempted bool     `json:"discoveryAttempted"`
	Discovered         int      `json:"discovered"`
	PendingDiscovery   int      `json:"pendingDiscovery"`
	Warnings           []string `json:"warnings"`
	Message            string   `json:"message"`
}

type CodexProbe struct {
	Configured        bool     `json:"configured"`
	DirectoryReadable bool     `json:"directoryReadable"`
	CandidateDetected bool     `json:"candidateDetected"`
	CandidateNames    []string `json:"candidateNames"`
	Platform          string   `json:"platform"`
	Message           string   `json:"message"`
}

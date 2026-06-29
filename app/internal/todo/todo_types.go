package todo

import "time"

const (
	todoStatusFile                = "_lists.json"
	todoScope                     = "Tasks.ReadWrite offline_access openid profile"
	todoSyncLocal                 = "local"
	todoSyncMicrosoft             = "microsoft"
	todoLocalTodoListID           = "local-todo"
	todoLocalGroceryListID        = "local-grocery"
	todoListOriginLocal           = "local"
	todoListOriginMicrosoft       = "microsoft"
	todoDashboardDockPreviewLimit = 12
	todoDashboardDockPerSlotLimit = 8
	// To Do cloud reconciliation is intentionally fixed at a bounded 25-second
	// cadence. Immediate list-open, local-write, and Sync now requests share the
	// same coordinator, so families never need to tune provider polling by hand.
	todoInboundSyncFixedSeconds = 25
	todoGraphDeltaMaxPages      = 100
)

type todoTokenStore struct {
	Account         string   `json:"account,omitempty"`
	ClientID        string   `json:"clientId,omitempty"`
	RefreshToken    string   `json:"refreshToken,omitempty"`
	AccessToken     string   `json:"accessToken,omitempty"`
	AccessExpiresAt int64    `json:"accessExpiresAt,omitempty"`
	Scopes          []string `json:"scopes,omitempty"`
	LinkedAt        int64    `json:"linkedAt,omitempty"`
}

type todoListInfo struct {
	ID            string `json:"id"`
	DisplayName   string `json:"displayName"`
	Origin        string `json:"origin,omitempty"`
	WellknownName string `json:"wellknownName,omitempty"`
}

type todoListsIndex struct {
	DeltaLink string         `json:"deltaLink,omitempty"`
	Lists     []todoListInfo `json:"lists"`
	UpdatedAt int64          `json:"updatedAt,omitempty"`
}

type todoTaskAssignment struct {
	PersonID           string `json:"personId"`
	PersonNameSnapshot string `json:"personNameSnapshot"`
	AssignedAt         int64  `json:"assignedAt"`
}

type todoTask struct {
	ID                   string              `json:"id"`
	Title                string              `json:"title"`
	Status               string              `json:"status"`
	Importance           string              `json:"importance,omitempty"`
	DueDateTime          map[string]any      `json:"dueDateTime,omitempty"`
	Body                 map[string]any      `json:"body,omitempty"`
	ChecklistItems       []todoChecklistItem `json:"checklistItems,omitempty"`
	LastModifiedDateTime string              `json:"lastModifiedDateTime,omitempty"`
	Pending              string              `json:"_pending,omitempty"`
	SyncFailed           bool                `json:"_syncFailed,omitempty"`
	CloudIgnored         bool                `json:"_cloudIgnored,omitempty"`
	ETag                 int64               `json:"_etag,omitempty"`
	DashGoAssignment     *todoTaskAssignment `json:"dashgoAssignment,omitempty"`
}

type todoChecklistItem struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	IsChecked   bool   `json:"isChecked"`
}

type todoPendingOp struct {
	Op         string         `json:"op"`
	ListID     string         `json:"listId"`
	TaskID     string         `json:"taskId,omitempty"`
	Payload    map[string]any `json:"payload,omitempty"`
	Attempts   int            `json:"attempts,omitempty"`
	Created    int64          `json:"created,omitempty"`
	Blocked    bool           `json:"blocked,omitempty"`
	LastError  string         `json:"lastError,omitempty"`
	LastStatus int            `json:"lastStatus,omitempty"`
}

type todoListCache struct {
	Version     int             `json:"version"`
	ListID      string          `json:"listId"`
	DisplayName string          `json:"displayName"`
	DeltaLink   string          `json:"deltaLink,omitempty"`
	Tasks       []todoTask      `json:"tasks"`
	PendingOps  []todoPendingOp `json:"pendingOps,omitempty"`
	LastSyncAt  int64           `json:"lastSyncAt,omitempty"`
	LastError   string          `json:"lastError,omitempty"`
}

// todoInboundSyncStatus reports the bounded server-owned Microsoft task pull
// state. It is runtime status, not a browser polling contract.
type todoInboundSyncStatus struct {
	ConfiguredSeconds int    `json:"configuredSeconds"`
	EffectiveSeconds  int    `json:"effectiveSeconds"`
	Mode              string `json:"mode"`
	Enabled           bool   `json:"enabled"`
	Running           bool   `json:"running"`
	Queued            bool   `json:"queued,omitempty"`
	QueueSeconds      int    `json:"queueSeconds,omitempty"`
	BackoffUntil      int64  `json:"backoffUntil,omitempty"`
	BackoffSeconds    int    `json:"backoffSeconds,omitempty"`
	LastSyncAt        int64  `json:"lastSyncAt,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	LastDurationMs    int64  `json:"lastDurationMs,omitempty"`
	LastQueueWaitMs   int64  `json:"lastQueueWaitMs,omitempty"`
	CoalescedRequests int    `json:"coalescedRequests,omitempty"`
}

type todoSyncListResult struct {
	ListID            string `json:"listId"`
	Title             string `json:"title"`
	Added             int    `json:"added"`
	Updated           int    `json:"updated"`
	Removed           int    `json:"removed"`
	LastSyncAt        int64  `json:"lastSyncAt"`
	DeltaMode         string `json:"deltaMode,omitempty"`
	Stage             string `json:"stage,omitempty"`
	Error             string `json:"error,omitempty"`
	QueueError        string `json:"queueError,omitempty"`
	QueueBlocked      int    `json:"queueBlocked,omitempty"`
	HTTPStatus        int    `json:"httpStatus,omitempty"`
	GraphCode         string `json:"graphCode,omitempty"`
	GraphInnerCode    string `json:"graphInnerCode,omitempty"`
	RequestID         string `json:"requestId,omitempty"`
	RetryAfterSeconds int    `json:"retryAfterSeconds,omitempty"`
}

type todoSyncResult struct {
	OK             bool                 `json:"ok"`
	AlreadyRunning bool                 `json:"alreadyRunning,omitempty"`
	Queued         bool                 `json:"queued,omitempty"`
	Skipped        bool                 `json:"skipped,omitempty"`
	Partial        bool                 `json:"partial,omitempty"`
	Reason         string               `json:"reason,omitempty"`
	Lists          []todoSyncListResult `json:"lists"`
	LastSyncAt     int64                `json:"lastSyncAt,omitempty"`
}

type todoBlockedWriteStatus struct {
	ListID string `json:"listId"`
	Title  string `json:"title"`
	Count  int    `json:"count"`
}

// todoDashboardDockSummary is a bounded local-cache view for the optional
// dashboard ticker. It never performs a provider refresh or changes list state.
type todoDashboardDockItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Assignee string `json:"assignee,omitempty"`
}

type todoDashboardDockSlot struct {
	Slot      string                  `json:"slot"`
	ListID    string                  `json:"listId"`
	Title     string                  `json:"title"`
	OpenCount int                     `json:"openCount"`
	Items     []todoDashboardDockItem `json:"items"`
}

type todoDashboardDockSummary struct {
	Enabled        bool                    `json:"enabled"`
	Slots          []todoDashboardDockSlot `json:"slots"`
	TotalOpenCount int                     `json:"totalOpenCount"`
}

type todoMigrationArchive struct {
	ID         string       `json:"id"`
	Source     todoListInfo `json:"source"`
	Reason     string       `json:"reason"`
	TaskCount  int          `json:"taskCount"`
	ArchivedAt int64        `json:"archivedAt"`
	ExpiresAt  int64        `json:"expiresAt"`
	PurgedAt   int64        `json:"purgedAt,omitempty"`
}

type todoMigrationArchiveIndex struct {
	Version int                    `json:"version"`
	Items   []todoMigrationArchive `json:"items"`
}

type todoMigrationArchiveSnapshot struct {
	Version int                  `json:"version"`
	Archive todoMigrationArchive `json:"archive"`
	Cache   todoListCache        `json:"cache"`
}

type todoAuthPending struct {
	State           string `json:"state"`
	UserCode        string `json:"userCode,omitempty"`
	VerificationURI string `json:"verificationUri,omitempty"`
	ExpiresAt       int64  `json:"expiresAt,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

func todoNowMillis() int64 { return time.Now().UnixMilli() }

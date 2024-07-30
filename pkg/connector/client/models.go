package client

type ConfluenceLink struct {
	Base string `json:"base"`
	Next string `json:"next,omitempty"`
}

type ConfluenceUser struct {
	AccountId   string                `json:"accountId"`
	AccountType string                `json:"accountType"`
	DisplayName string                `json:"displayName"`
	Email       string                `json:"email,omitempty"`
	Operations  []ConfluenceOperation `json:"operations,omitempty"`
}

type ConfluenceOperation struct {
	Operation  string `json:"operation"`
	TargetType string `json:"targetType"`
}

type confluenceUserList struct {
	Start   int              `json:"start"`
	Limit   int              `json:"limit"`
	Size    int              `json:"size"`
	Links   ConfluenceLink   `json:"_links"`
	Results []ConfluenceUser `json:"results"`
}

type ConfluenceSearch struct {
	EntityType string         `json:"entityType"`
	Score      float64        `json:"score"`
	Title      string         `json:"title"`
	User       ConfluenceUser `json:"user"`
}

type ConfluenceSearchList struct {
	Start     int                `json:"start"`
	Limit     int                `json:"limit"`
	TotalSize int                `json:"totalSize"`
	Size      int                `json:"size"`
	Results   []ConfluenceSearch `json:"results"`
}

type ConfluenceGroup struct {
	Type string
	Name string
	Id   string
}

type confluenceGroupList struct {
	Start   int               `json:"start"`
	Limit   int               `json:"limit"`
	Size    int               `json:"size"`
	Links   ConfluenceLink    `json:"_links"`
	Results []ConfluenceGroup `json:"results"`
}

type AddUserToGroupRequestBody struct {
	AccountId string `json:"accountId"`
}

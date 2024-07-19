package client

type ConfluenceUser struct {
	AccountId   string `json:"accountId"`
	AccountType string `json:"accountType"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type confluenceUserList struct {
	Start   int              `json:"start"`
	Limit   int              `json:"limit"`
	Size    int              `json:"size"`
	Links   ConfluenceLink   `json:"_links"`
	Results []ConfluenceUser `json:"results"`
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

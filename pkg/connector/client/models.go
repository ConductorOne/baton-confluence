package client

type ConfluenceUser struct {
	AccountId   string
	AccountType string
	DisplayName string
	Email       string
}

type confluenceUserList struct {
	Start   int
	Limit   int
	Size    int
	Results []ConfluenceUser
}

type ConfluenceGroup struct {
	Type string
	Name string
	Id   string
}

type confluenceGroupList struct {
	Start   int
	Limit   int
	Size    int
	Results []ConfluenceGroup
}

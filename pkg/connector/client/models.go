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

type ConfluenceSpaceDescriptionValue struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

type ConfluenceSpaceDescription struct {
	Plain ConfluenceSpaceDescriptionValue `json:"plain"`
}

type ConfluenceMeta struct {
	HasMore bool   `json:"hasMore"`
	Cursor  string `json:"cursor"`
}

type ConfluenceSpaceOperationsResponse struct {
	Links   ConfluenceLink             `json:"_links"`
	Meta    ConfluenceMeta             `json:"meta"`
	Results []ConfluenceSpaceOperation `json:"results"`
}

type ConfluenceSpacePermissionResponse struct {
	Links   ConfluenceLink              `json:"_links"`
	Results []ConfluenceSpacePermission `json:"results"`
}

type ConfluenceSpacePermission struct {
	Id        string                             `json:"id"`
	Principal ConfluenceSpacePermissionPrincipal `json:"principal"`
	Operation ConfluenceSpacePermissionOperation `json:"operation"`
}

type ConfluenceSpacePermissionPrincipal struct {
	Id   string `json:"id"`
	Type string `json:"type"`
}

type ConfluenceSpaceOperation struct {
	Operation  string `json:"operation"`
	TargetType string `json:"targetType"`
}

type ConfluenceSpacePermissionOperation struct {
	Key        string `json:"key"`
	TargetType string `json:"targetType"`
}

type ConfluenceSpace struct {
	AuthorId    string                            `json:"authorId"`
	CreatedAt   string                            `json:"createdAt"`
	Description ConfluenceSpaceDescription        `json:"description"`
	HomepageId  string                            `json:"homepageId"`
	Icon        string                            `json:"icon"`
	Id          string                            `json:"id"`
	Key         string                            `json:"key"`
	Name        string                            `json:"name"`
	Operations  ConfluenceSpaceOperationsResponse `json:"operations"`
	Status      string                            `json:"status"`
	Type        string                            `json:"type"`
}

type confluenceSpaceList struct {
	Links   ConfluenceLink    `json:"_links"`
	Results []ConfluenceSpace `json:"results"`
}

type SpacePermissionSubject struct {
	Type       string `json:"type"`
	Identifier string `json:"identifier"`
}

type SpacePermissionOperation struct {
	Key    string `json:"key"`
	Target string `json:"target"`
}

type CreateSpacePermissionRequestBody struct {
	Subject   SpacePermissionSubject   `json:"subject"`
	Operation SpacePermissionOperation `json:"operation"`
}

type AddUserToGroupRequestBody struct {
	AccountId string `json:"accountId"`
}

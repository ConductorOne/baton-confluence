package connector

import (
	v2 "github.com/conductorone/baton-sdk/pb/c1/connector/v2"
	"github.com/conductorone/baton-sdk/pkg/annotations"
)

const (
	resourceTypeGroupID = "group"
	resourceTypeUserID  = "user"
	resourceTypeSpaceID = "space"

	SpaceRoleResourceTypeID           = "space_role"
	SpaceRoleAssignmentResourceTypeID = "space_role_assignment"
)

var (
	resourceTypeGroup = &v2.ResourceType{
		Id:          resourceTypeGroupID,
		DisplayName: "Group",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_GROUP},
	}
	resourceTypeUser = &v2.ResourceType{
		Id:          resourceTypeUserID,
		DisplayName: "User",
		Traits: []v2.ResourceType_Trait{
			v2.ResourceType_TRAIT_USER,
		},
		Annotations: annotationsForUserResourceType(),
	}
	spaceResourceType = &v2.ResourceType{
		Id:          resourceTypeSpaceID,
		DisplayName: "Space",
		Traits:      []v2.ResourceType_Trait{},
	}
	spaceRoleResourceType = &v2.ResourceType{
		Id:          SpaceRoleResourceTypeID,
		DisplayName: "Space Role",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_ROLE},
		Annotations: annotations.New(&v2.OptInRequired{}),
	}
	spaceRoleAssignmentResourceType = &v2.ResourceType{
		Id:          SpaceRoleAssignmentResourceTypeID,
		DisplayName: "Space Role Assignment",
		Traits:      []v2.ResourceType_Trait{v2.ResourceType_TRAIT_SCOPE_BINDING},
		Annotations: spaceRoleAssignmentsAnnotations(),
	}
)

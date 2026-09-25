package server

import (
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
)

// feature_option is a single feature tag offered in the developer dashboard.
type feature_option struct {
	ID          string
	Description string
}

// feature_group collects the feature tags of one category.
type feature_group struct {
	NameKey string
	Options []feature_option
}

// buildFeatureGroups returns the feature tags in the order they are offered to
// developers. The "review" tag is left out because it is a status set by the
// server rather than a feature the developer can claim.
func buildFeatureGroups() []feature_group {
	groups := []struct {
		name_key string
		tags     []string
	}{
		{"developer_feature_group_features", []string{"achievements", "config", "controllers", "dlc", "legacy", "matchmaking", "save", "points"}},
		{"developer_feature_group_rating", []string{"everyone", "older", "mature"}},
		{"developer_feature_group_platform", []string{"multidev", "mobile"}},
		{"developer_feature_group_source", []string{"oss", "proprietary"}},
		{"developer_feature_group_made_with", []string{"ontw", "onpm", "oneq", "onscratch"}},
		{"developer_feature_group_advisory", []string{"violent", "substances"}},
		{"developer_feature_group_connectivity", []string{"call", "vchat", "mail", "vmail"}},
	}

	loaded_groups := []feature_group{}
	for _, group := range groups {
		entry := feature_group{NameKey: group.name_key}
		for _, tag := range group.tags {
			if description, ok := types.GameFeatureTags[tag]; ok {
				entry.Options = append(entry.Options, feature_option{ID: tag, Description: description})
			}
		}
		loaded_groups = append(loaded_groups, entry)
	}

	return loaded_groups
}

func (s *Server) DeveloperDashboard(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Please login first before accessing the developer dashboard.",
		})
	}

	claims := s.Authorization.GetNormalClaims(c)

	user, err := s.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to retrieve user data.",
		})
	}

	var avatarURL string
	if user != nil && user.Avatar != nil {
		avatarURL = user.Avatar.Link
	} else {
		avatarURL = "/assets/static/img/ui/placeholder_user.png"
	}

	data := map[string]any{
		"BaseURL":       s.ServerURL,
		"ServerName":    s.ServerName,
		"Username":      claims.Username,
		"AvatarURL":     avatarURL,
		"LoggedIn":      true,
		"IsAdmin":       s.IsAdmin(c),
		"FeatureGroups": buildFeatureGroups(),
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/developer", data, "views/layouts/nofooter")
}

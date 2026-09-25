package server

import (
	"github.com/cloudlink-omega/accounts/pkg/constants"
	"github.com/gofiber/fiber/v2"
)

func (s *Server) Security(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Please login first before accessing the security page.",
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
	if user.Avatar != nil {
		avatarURL = user.Avatar.Link
	} else {
		avatarURL = "/assets/static/img/ui/placeholder_user.png"
	}

	// Get TOTP status
	totpEnabled := user.State.Read(constants.USER_IS_TOTP_ENABLED)
	emailVerified := user.State.Read(constants.USER_IS_ACTIVE)

	data := map[string]any{
		"BaseURL":        s.ServerURL,
		"Username":       claims.Username,
		"ServerName":     s.ServerName,
		"LoggedIn":       true,
		"AvatarURL":      avatarURL,
		"Title":          "Security Settings",
		"User":           user,
		"TOTPEnabled":    totpEnabled,
		"EmailVerified":  emailVerified,
		"OAuthOnly":      user.State.Read(constants.USER_IS_OAUTH_ONLY),
		"IsAdmin":        s.IsAdmin(c),
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/security", data, "views/layouts/nofooter")
}

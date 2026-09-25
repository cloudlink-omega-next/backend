package server

import (
	"github.com/gofiber/fiber/v2"
)

func (s *Server) Profile(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Please login first before accessing the profile page.",
		})
	}

	claims := s.Authorization.GetNormalClaims(c)

	user, err := s.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to retrieve user profile.",
		})
	}

	var avatarURL string
	if user.Avatar != nil {
		avatarURL = user.Avatar.Link
	} else {
		avatarURL = "/assets/static/img/ui/placeholder_user.png"
	}

	data := map[string]any{
		"BaseURL":    s.ServerURL,
		"Username":   claims.Username,
		"ServerName": s.ServerName,
		"LoggedIn":   true,
		"AvatarURL":  avatarURL,
		"Title":      "Profile",
		"User":       user,
		"IsAdmin":    s.IsAdmin(c),
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/profile", data, "views/layouts/nofooter")
}

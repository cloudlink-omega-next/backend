package server

import (
	"github.com/gofiber/fiber/v2"
)

func (s *Server) ReportNew(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Please login first before submitting a report.",
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
		"BaseURL":    s.ServerURL,
		"ServerName": s.ServerName,
		"LoggedIn":   true,
		"Username":   claims.Username,
		"AvatarURL":  avatarURL,
		"IsAdmin":    s.IsAdmin(c),
		"Title":      "Submit Report",
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/reports", data, "views/layouts/default")
}

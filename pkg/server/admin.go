package server

import (
	"github.com/cloudlink-omega/accounts/pkg/constants"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
)

func (s *Server) IsAdmin(c *fiber.Ctx) bool {
	if s.AdminEmail == "" {
		return false
	}

	claims := s.Authorization.GetNormalClaims(c)
	if claims == nil {
		return false
	}

	if claims.Email == s.AdminEmail {
		return true
	}

	user, err := s.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return false
	}

	return user.State.Read(constants.USER_IS_ADMIN)
}

func (s *Server) Admin(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Nope, sorry.",
		})
	}

	if !s.IsAdmin(c) {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusForbidden,
			Message: "Nope, sorry.",
		})
	}

	claims := s.Authorization.GetNormalClaims(c)

	var avatarURL string
	var user *types.User
	if err := s.Accounts.DB.DB.Preload("Avatar").First(&user, "id = ?", claims.ULID).Error; err == nil && user != nil && user.Avatar != nil {
		avatarURL = user.Avatar.Link
	}
	if avatarURL == "" {
		avatarURL = "/assets/static/img/ui/placeholder_user.png"
	}

	data := map[string]any{
		"BaseURL":    s.ServerURL,
		"ServerName": s.ServerName,
		"Username":   claims.Username,
		"LoggedIn":   true,
		"IsAdmin":    true,
		"AvatarURL":  avatarURL,
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/admin", data, "views/layouts/nofooter")
}

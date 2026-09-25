package server

import (
	"github.com/gofiber/fiber/v2"
)

func (s *Server) Friends(c *fiber.Ctx) error {
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
		"LoggedIn":   claims != nil,
		"Username":   "",
		"ServerName": s.ServerName,
		"Title":      "Friends",
		"AvatarURL":  avatarURL,
		"IsAdmin":    s.IsAdmin(c),
	}
	if claims != nil {
		data["Username"] = claims.Username
	}
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/friends", data, "views/layouts/default")
}

func (s *Server) FriendRequests(c *fiber.Ctx) error {
	claims := s.Authorization.GetNormalClaims(c)
	data := map[string]any{
		"BaseURL":    s.ServerURL,
		"LoggedIn":   claims != nil,
		"Username":   "",
		"ServerName": s.ServerName,
		"Title":      "Friend Requests",
		"IsAdmin":    s.IsAdmin(c),
	}
	if claims != nil {
		data["Username"] = claims.Username
	}
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/friend_requests", data, "views/layouts/default")
}

func (s *Server) SearchFriends(c *fiber.Ctx) error {
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
		"LoggedIn":   claims != nil,
		"Username":   "",
		"ServerName": s.ServerName,
		"Title":      "Find Friends",
		"AvatarURL":  avatarURL,
		"IsAdmin":    s.IsAdmin(c),
	}
	if claims != nil {
		data["Username"] = claims.Username
	}
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/search_users", data, "views/layouts/default")
}

func (s *Server) Chat(c *fiber.Ctx) error {
	friendID := c.Params("user_id")
	claims := s.Authorization.GetNormalClaims(c)

	var avatarURL string
	if claims != nil {
		user, err := s.Accounts.DB.GetUser(claims.ULID)
		if err == nil && user.Avatar != nil {
			avatarURL = user.Avatar.Link
		} else {
			avatarURL = "/assets/static/img/ui/placeholder_user.png"
		}
	}

	var friendAvatarURL string
	var friendName string
	if friendID != "" && claims != nil {
		friendUser, err := s.Accounts.DB.GetUser(friendID)
		if err == nil {
			if friendUser.Avatar != nil && friendUser.Avatar.Link != "" {
				friendAvatarURL = friendUser.Avatar.Link
			} else {
				friendAvatarURL = "/assets/static/img/ui/placeholder_user.png"
			}
			if friendUser.Name != "" {
				friendName = friendUser.Name
			} else {
				friendName = friendUser.Username
			}
		} else {
			friendAvatarURL = "/assets/static/img/ui/placeholder_user.png"
		}
	}

	data := map[string]any{
		"BaseURL":         s.ServerURL,
		"LoggedIn":        claims != nil,
		"Username":        "",
		"ServerName":      s.ServerName,
		"Title":           "Chat",
		"FriendID":        friendID,
		"FriendName":      friendName,
		"AvatarURL":       avatarURL,
		"FriendAvatarURL": friendAvatarURL,
		"IsAdmin":         s.IsAdmin(c),
	}
	if claims != nil {
		data["Username"] = claims.Username
	}
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/chat", data, "views/layouts/default")
}

func (s *Server) Blocklist(c *fiber.Ctx) error {
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
		"LoggedIn":   claims != nil,
		"Username":   "",
		"ServerName": s.ServerName,
		"Title":      "Blocked Users",
		"AvatarURL":  avatarURL,
		"IsAdmin":    s.IsAdmin(c),
	}
	if claims != nil {
		data["Username"] = claims.Username
	}
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/blocklist", data, "views/layouts/default")
}

func (s *Server) Notifications(c *fiber.Ctx) error {
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
		"LoggedIn":   claims != nil,
		"Username":   "",
		"ServerName": s.ServerName,
		"Title":      "Notifications",
		"AvatarURL":  avatarURL,
		"IsAdmin":    s.IsAdmin(c),
	}
	if claims != nil {
		data["Username"] = claims.Username
	}
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/notifications", data, "views/layouts/default")
}

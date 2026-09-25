package server

import (
	"os"
	"time"

	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
)

// Handler for the explore page
func (s *Server) Play(c *fiber.Ctx) error {

	// Read ID path
	id := c.Params("id")

	// Check if the game exists
	game := s.DB.GetGame(id)

	_, file_error := os.Stat(s.HostedPath + "/projects_public/" + id)

	claims := s.Authorization.GetNormalClaims(c)
	loggedIn := s.Authorization.ValidFromNormal(c)
	var username string
	avatarURL := "/assets/static/img/ui/placeholder_user.png"
	if loggedIn {
		username = claims.Username

		if user, err := s.Accounts.DB.GetUser(claims.ULID); err == nil && user != nil && user.Avatar != nil {
			avatarURL = user.Avatar.Link
		}
	}

	if file_error != nil || game == nil {
		if file_error != nil {
			log.Error(file_error)
		}
		data := map[string]any{
			"BaseURL":    s.ServerURL,
			"LoggedIn":   loggedIn,
			"Username":   username,
			"AvatarURL":  avatarURL,
			"ServerName": s.ServerName,
			"Title":      "Whoops!",
		}
		c.Context().SetContentType("text/html; charset=utf-8")
		c.Status(fiber.StatusNotFound)
		return c.Render("views/play_not_found", data, "views/layouts/default")
	}

	if loggedIn && game != nil {
		now := time.Now()
		s.DB.DB.FirstOrCreate(&types.UserPlayedGame{
			UserID:          claims.ULID,
			DeveloperGameID: game.ID,
		}, map[string]any{
			"user_id":          claims.ULID,
			"developer_game_id": game.ID,
		}).Updates(map[string]any{
			"updated_at": now,
		})
	}

	data := map[string]any{
		"BaseURL":           s.ServerURL,
		"LoggedIn":          loggedIn,
		"Username":          username,
		"AvatarURL":         avatarURL,
		"ServerName":        s.ServerName,
		"Title":             game.Name,
		"GameName":          game.Name,
		"DeveloperName":     game.Developer.Name,
		"DeveloperAvatarURL": func() string {
			if game.Developer.Avatar != nil && game.Developer.Avatar.Link != "" {
				return game.Developer.Avatar.Link
			}
			return "/assets/static/img/ui/placeholder_user.png"
		}(),
		"GameDescription": game.Description,
		"ID":              game.ID,
		"Features":        game.Features,
		/* "Comments": []map[string]string{
			{
				"ID":       "1",
				"Username": "MikeDEV",
				"Comment":  `Hello world! This is an example comment. **Very cool!** *This should render as markdown.*`,
				"Date":     "1/1/2023",
			},
		}, */
		"IsAdmin": s.IsAdmin(c),
	}

	// Render the modal template
	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/play", data, "views/layouts/default")
}

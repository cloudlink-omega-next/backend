package server

import (
	"time"

	"github.com/cloudlink-omega/storage/pkg/common"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UserGameSaveView struct {
	SaveSlot      uint8
	DeveloperGame *types.DeveloperGame
	SaveData      string
	UpdatedAt     time.Time
}

func (s *Server) CloudSaves(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusUnauthorized,
			Message: "Please login first before accessing cloud saves.",
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

	saves, err := s.GetUserCloudSaves(claims.ULID)
	if err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusInternalServerError,
			Message: "Failed to retrieve cloud saves: " + err.Error(),
		})
	}

	data := map[string]any{
		"BaseURL":    s.ServerURL,
		"Username":   claims.Username,
		"ServerName": s.ServerName,
		"LoggedIn":   true,
		"AvatarURL":  avatarURL,
		"Title":      "Cloud Saves",
		"User":       user,
		"Saves":      saves,
		"IsAdmin":    s.IsAdmin(c),
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/cloud_saves", data, "views/layouts/nofooter")
}

type cloudSaveRow struct {
	SaveSlot          uint8
	SaveData          string
	UpdatedAt         time.Time
	DeveloperGameID   string
	DeveloperGameName string
	GameDescription   string
	ThumbnailLink     string
}

func (s *Server) GetUserCloudSaves(userID string) ([]*UserGameSaveView, error) {
	var rows []cloudSaveRow
	err := s.DB.DB.Table("user_game_saves").
		Select("user_game_saves.save_slot, user_game_saves.save_data, user_game_saves.updated_at, developer_games.id AS developer_game_id, developer_games.name AS developer_game_name, developer_games.description AS developer_game_description, images.link AS developer_game_thumbnail_link").
		Joins("LEFT JOIN developer_games ON developer_games.id = user_game_saves.developer_game_id").
		Joins("LEFT JOIN images ON images.id = developer_games.thumbnail_id").
		Where("user_game_saves.user_id = ?", userID).
		Order("user_game_saves.developer_game_id ASC, user_game_saves.save_slot ASC").
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	result := make([]*UserGameSaveView, 0, len(rows))
	for _, row := range rows {
		saveData := row.SaveData
		if saveData == "" {
			saveData = "null"
		}

		view := &UserGameSaveView{
			SaveSlot:  row.SaveSlot,
			SaveData:  saveData,
			UpdatedAt: row.UpdatedAt,
			DeveloperGame: &types.DeveloperGame{
				ID:          row.DeveloperGameID,
				Name:        row.DeveloperGameName,
				Description: row.GameDescription,
				Thumbnail: &types.Image{
					Link: row.ThumbnailLink,
				},
			},
		}

		if view.DeveloperGame.Thumbnail == nil {
			view.DeveloperGame.Thumbnail = &types.Image{}
		}

		result = append(result, view)
	}

	return result, nil
}

type CloudSaveDeleteRequest struct {
	SaveSlot       uint8  `json:"save_slot" validate:"required,min=1,max=10"`
	DeveloperGameID string `json:"developer_game_id" validate:"required"`
}

func (s *Server) DeleteCloudSave(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"result": "Unauthorized.",
		})
	}

	claims := s.Authorization.GetNormalClaims(c)
	var req CloudSaveDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"result": "Invalid request body.",
		})
	}

	result := s.DB.DB.Where("user_id = ? AND save_slot = ? AND developer_game_id = ?", claims.ULID, req.SaveSlot, req.DeveloperGameID).Delete(&types.UserGameSave{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": result.Error.Error(),
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"result": "Save slot not found.",
		})
	}

	_ = common.LogEvent(s.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "game_save_deleted",
		Details:    "Cloud save deleted",
		Successful: true,
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"result": "Cloud save deleted successfully.",
	})
}

func (s *Server) CloudSaveDecrypt(c *fiber.Ctx) error {
	loggedIn := s.Authorization.ValidFromNormal(c)
	if !loggedIn {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"result": "Unauthorized.",
		})
	}

	claims := s.Authorization.GetNormalClaims(c)
	user, err := s.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": "Failed to get user.",
		})
	}

	var req CloudSaveDeleteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"result": "Invalid request body.",
		})
	}

	var slot types.UserGameSave
	result := s.DB.DB.First(&slot, "user_id = ? AND save_slot = ? AND developer_game_id = ?", claims.ULID, req.SaveSlot, req.DeveloperGameID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"result": "Save slot not found.",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": result.Error.Error(),
		})
	}

	decrypted, err := s.Accounts.DB.Decrypt(user, slot.SaveData)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"result": "OK",
		"data":   decrypted,
	})
}

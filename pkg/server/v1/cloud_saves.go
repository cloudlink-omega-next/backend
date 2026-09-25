package v1

import (
	"github.com/cloudlink-omega/accounts/pkg/structs"
	"github.com/cloudlink-omega/storage/pkg/common"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CloudSaveRequest struct {
	SaveSlot       uint8  `json:"save_slot" validate:"required,min=1,max=10"`
	DeveloperGameID string `json:"developer_game_id" validate:"required"`
}

func (a *APIv1) GetCloudSaves(c *fiber.Ctx) error {
	var claims *structs.Claims
	if a.ParentServer.Authorization.ValidFromNormal(c) {
		claims = a.ParentServer.Authorization.GetNormalClaims(c)
	} else {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var rows []struct {
		SaveSlot          uint8
		SaveData          string
		UpdatedAt         string
		DeveloperGameID   string
		DeveloperGameName string
		GameDescription   string
		ThumbnailLink     string
	}

	err := a.ParentServer.DB.DB.Table("user_game_saves").
		Select("user_game_saves.save_slot, user_game_saves.save_data, user_game_saves.updated_at, developer_games.id AS developer_game_id, developer_games.name AS developer_game_name, developer_games.description AS developer_game_description, images.link AS developer_game_thumbnail_link").
		Joins("LEFT JOIN developer_games ON developer_games.id = user_game_saves.developer_game_id").
		Joins("LEFT JOIN images ON images.id = developer_games.thumbnail_id").
		Where("user_game_saves.user_id = ?", claims.ULID).
		Order("user_game_saves.developer_game_id ASC, user_game_saves.save_slot ASC").
		Scan(&rows).Error

	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type responseSave struct {
		SaveSlot      uint8                `json:"save_slot"`
		DeveloperGame *types.DeveloperGame `json:"developer_game"`
		SaveData      string               `json:"save_data"`
		UpdatedAt     string               `json:"updated_at"`
	}

	response := make([]responseSave, 0, len(rows))
	for _, row := range rows {
		saveData := row.SaveData
		if saveData == "" {
			saveData = "null"
		}

		response = append(response, responseSave{
			SaveSlot: row.SaveSlot,
			DeveloperGame: &types.DeveloperGame{
				ID:          row.DeveloperGameID,
				Name:        row.DeveloperGameName,
				Description: row.GameDescription,
				Thumbnail: &types.Image{
					Link: row.ThumbnailLink,
				},
			},
			SaveData:  saveData,
			UpdatedAt: row.UpdatedAt,
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", response)
}

func (a *APIv1) CloudSaveDecrypt(c *fiber.Ctx) error {
	var req CloudSaveRequest
	if err := c.BodyParser(&req); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if req.SaveSlot < 1 || req.SaveSlot > 10 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid save slot (must be a number between 1-10).", nil)
	}

	var claims *structs.Claims
	if a.ParentServer.Authorization.ValidFromNormal(c) {
		claims = a.ParentServer.Authorization.GetNormalClaims(c)
	} else {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to get user.", nil)
	}

	var slot types.UserGameSave
	result := a.ParentServer.DB.DB.First(&slot, "user_id = ? AND save_slot = ? AND developer_game_id = ?", claims.ULID, req.SaveSlot, req.DeveloperGameID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Save slot not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, result.Error.Error(), nil)
	}

	decrypted, err := a.ParentServer.Accounts.DB.Decrypt(user, slot.SaveData)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", decrypted)
}

func (a *APIv1) DeleteCloudSave(c *fiber.Ctx) error {
	var req CloudSaveRequest
	if err := c.BodyParser(&req); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if req.SaveSlot < 1 || req.SaveSlot > 10 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid save slot (must be a number between 1-10).", nil)
	}

	var claims *structs.Claims
	if a.ParentServer.Authorization.ValidFromNormal(c) {
		claims = a.ParentServer.Authorization.GetNormalClaims(c)
	} else {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	result := a.ParentServer.DB.DB.Where("user_id = ? AND save_slot = ? AND developer_game_id = ?", claims.ULID, req.SaveSlot, req.DeveloperGameID).Delete(&types.UserGameSave{})
	if result.Error != nil {
		return APIResult(c, fiber.StatusInternalServerError, result.Error.Error(), nil)
	}

	if result.RowsAffected == 0 {
		return APIResult(c, fiber.StatusNotFound, "Save slot not found.", nil)
	}

	_ = common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "game_save_deleted",
		Details:    "Cloud save deleted",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Cloud save deleted successfully.", nil)
}

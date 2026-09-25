package v1

import (
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

// TriggerAchievement creates a new achievement for the current user
func (a *APIv1) TriggerAchievement(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var req struct {
		GameID      string `json:"game_id"`
		Description string `json:"description"`
		Points      uint64 `json:"points"`
		IconID      string `json:"icon_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if req.GameID == "" || req.Description == "" {
		return APIResult(c, fiber.StatusBadRequest, "game_id and description are required.", nil)
	}

	// Check if user already earned this achievement
	var earnedCount int64
	if err := a.Database.DB.Model(&types.Achievement{}).
		Where("user_id = ? AND developer_game_id = ? AND description = ?", claims.ULID, req.GameID, req.Description).
		Count(&earnedCount).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if earnedCount > 0 {
		return APIResult(c, fiber.StatusConflict, "Achievement already earned.", nil)
	}

	achievement := &types.Achievement{
		ID:              ulid.Make().String(),
		UserID:          claims.ULID,
		DeveloperGameID: req.GameID,
		Description:     req.Description,
		Points:          req.Points,
	}

	if req.IconID != "" {
		achievement.IconID = &req.IconID
	}

	if err := a.Database.DB.Create(achievement).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	// Preload related data for response
	var created types.Achievement
	if err := a.Database.DB.Preload("DeveloperGame").Preload("Icon").First(&created, "id = ?", achievement.ID).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", created)
}

// GetAchievements returns all achievements for the current user
func (a *APIv1) GetAchievements(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var achievements []*types.Achievement
	if err := a.Database.DB.Preload("DeveloperGame").Preload("Icon").Where("user_id = ?", claims.ULID).Order("created_at DESC").Find(&achievements).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", achievements)
}

// GetAchievementsByGame returns achievements for a specific game
func (a *APIv1) GetAchievementsByGame(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	gameID := c.Params("game_id")
	if gameID == "" {
		return APIResult(c, fiber.StatusBadRequest, "game_id is required.", nil)
	}

	var achievements []*types.Achievement
	if err := a.Database.DB.Preload("DeveloperGame").Preload("Icon").Where("user_id = ? AND developer_game_id = ?", claims.ULID, gameID).Order("created_at DESC").Find(&achievements).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", achievements)
}

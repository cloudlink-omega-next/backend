package v1

import (
	"github.com/cloudlink-omega/storage/pkg/common"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
)

type GetSettingsResponse struct {
	Language  string `json:"language"`
	Theme     string `json:"theme"`
	IsPublic  bool   `json:"is_public"`
}

type UpdateSettingsRequest struct {
	Language  string `json:"language"`
	Theme     string `json:"theme"`
	IsPublic  *bool  `json:"is_public"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

func (a *APIv1) GetSettings(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve settings.", nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", GetSettingsResponse{
		Language: user.Language,
		Theme:    user.Theme,
		IsPublic: user.IsPublic,
	})
}

func (a *APIv1) UpdateSettings(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var req UpdateSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	updateData := map[string]any{}

	if req.Language != "" {
		updateData["language"] = req.Language
		user.Language = req.Language
	}

	if req.Theme != "" {
		validThemes := map[string]bool{
			"light":  true,
			"dark":   true,
			"system": true,
		}
		if !validThemes[req.Theme] {
			return APIResult(c, fiber.StatusBadRequest, "Invalid theme. Must be light, dark, or system.", nil)
		}
		updateData["theme"] = req.Theme
		user.Theme = req.Theme
	}

	if req.IsPublic != nil {
		updateData["is_public"] = *req.IsPublic
		user.IsPublic = *req.IsPublic
	}

	if len(updateData) > 0 {
		if err := a.ParentServer.Accounts.DB.DB.Model(&types.User{}).Where("id = ?", user.ID).Updates(updateData).Error; err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to update settings.", nil)
		}
	}

	_ = common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "user_settings_updated",
		Details:    "User settings updated",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "OK", GetSettingsResponse{
		Language: user.Language,
		Theme:    user.Theme,
		IsPublic: user.IsPublic,
	})
}

func (a *APIv1) ChangePassword(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if len(req.NewPassword) < 8 {
		return APIResult(c, fiber.StatusBadRequest, "New password must be at least 8 characters.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	if !a.ParentServer.Accounts.DB.VerifyPassword(user, req.CurrentPassword) {
		return APIResult(c, fiber.StatusUnauthorized, "Current password is incorrect.", nil)
	}

	newPasswordHash, err := a.ParentServer.Accounts.DB.HashPassword(req.NewPassword)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to hash new password.", nil)
	}

	if err := a.ParentServer.DB.DB.Model(user).Update("password", newPasswordHash).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to update password.", nil)
	}

	_ = common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "user_password_changed",
		Details:    "User changed password",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Password updated successfully.", nil)
}

func (a *APIv1) ExportAccountData(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	var gameSaves []*types.UserGameSave
	if err := a.ParentServer.DB.DB.Where("user_id = ?", claims.ULID).Find(&gameSaves).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve game saves.", nil)
	}

	var events []*types.UserEvent
	if err := a.ParentServer.DB.DB.Where("user_id = ?", claims.ULID).Order("created_at DESC").Limit(100).Find(&events).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve events.", nil)
	}

	var sessions []*types.UserSession
	if err := a.ParentServer.Accounts.DB.DB.Where("user_id = ?", claims.ULID).Find(&sessions).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve sessions.", nil)
	}

	exportData := map[string]any{
		"user": map[string]any{
			"id":        user.ID,
			"username":  user.Username,
			"email":     user.Email,
			"name":      user.Name,
			"bio":       user.Bio,
			"location":  user.Location,
			"website":   user.Website,
			"language":  user.Language,
			"theme":     user.Theme,
			"is_public": user.IsPublic,
			"created_at": user.CreatedAt,
			"updated_at": user.UpdatedAt,
		},
		"game_saves": gameSaves,
		"events":     events,
		"sessions":   sessions,
	}

	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", "attachment; filename=account-data.json")
	return c.JSON(exportData)
}

func (a *APIv1) DeleteAccount(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	tx := a.ParentServer.DB.DB.Begin()

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserGameSave{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete game saves.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserEvent{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete events.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserSession{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete sessions.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.Friend{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete friends.", nil)
	}

	if err := tx.Where("sender_id = ? OR receiver_id = ?", claims.ULID, claims.ULID).Delete(&types.FriendRequest{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete friend requests.", nil)
	}

	if err := tx.Where("user_id = ? OR blocked_id = ?", claims.ULID, claims.ULID).Delete(&types.Blocklist{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete blocklist entries.", nil)
	}

	if err := tx.Where("sender_id = ? OR receiver_id = ?", claims.ULID, claims.ULID).Delete(&types.Message{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete messages.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.Notification{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete notifications.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserPlayedGame{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete played games.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.Achievement{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete achievements.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.GameComment{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete comments.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.DeveloperMember{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete developer memberships.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserGoogle{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete Google account.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserDiscord{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete Discord account.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserGitHub{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete GitHub account.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.UserTOTP{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete TOTP settings.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.Verification{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete verifications.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.EmailChangeToken{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete email change tokens.", nil)
	}

	if err := tx.Where("user_id = ?", claims.ULID).Delete(&types.RecoveryCode{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete recovery codes.", nil)
	}

	if err := tx.Delete(user).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete user.", nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to commit transaction.", nil)
	}

	// The account no longer exists, so this cannot be recorded as a user event:
	// the row would violate the user_events foreign key and would be cascaded
	// away anyway. Record it as a system event to keep the audit trail.
	common.LogEvent(a.ParentServer.DB.DB, &types.SystemEvent{
		EventID:    "user_account_deleted",
		Details:    "User deleted their own account (" + user.Username + " / " + claims.ULID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Account deleted successfully.", nil)
}

package v1

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudlink-omega/accounts/pkg/constants"
	"github.com/cloudlink-omega/storage/pkg/bitfield"
	"github.com/cloudlink-omega/storage/pkg/common"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

func (a *APIv1) GetAdminOverview(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	var registeredUsers int64
	a.Database.DB.Model(&types.User{}).Count(&registeredUsers)

	var activeSessions int64
	a.Database.DB.Model(&types.UserSession{}).Where("expires_at > ?", time.Now()).Count(&activeSessions)

	var publishedGames int64
	var bitfield_bitfield bitfield.Bitfield8
	bitfield_bitfield.ManySet(constants.GAME_IS_ACTIVE, constants.GAME_IS_VERIFIED)
	a.Database.DB.Model(&types.DeveloperGame{}).Where("state >= ?", bitfield_bitfield).Count(&publishedGames)

	data := map[string]any{
		"registered_users":        registeredUsers,
		"active_sessions":         activeSessions,
		"active_lobbies":          0,
		"active_voice_connections": 0,
		"published_games":          publishedGames,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

func (a *APIv1) GetAdminLogs(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	var userEvents []*types.UserEvent
	if err := a.Database.DB.Preload("Event").Order("created_at DESC").Limit(limit).Offset(offset).Find(&userEvents).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var logs = []map[string]any{}
	for _, event := range userEvents {
		logs = append(logs, map[string]any{
			"timestamp":   event.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"class":       "User",
			"description": event.Event.Description,
			"status": func() string {
				if event.Successful {
					return "successful"
				}
				return "failed"
			}(),
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", logs)
}

func (a *APIv1) GetAdminAccounts(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	search := c.Query("search", "")

	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	var users []*types.User
	query := a.Database.DB.Preload("Avatar").Order("created_at DESC").Limit(limit).Offset(offset)

	if search != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Find(&users).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var count int64
	a.Database.DB.Model(&types.User{}).Count(&count)

	var accounts = []map[string]any{}
	for _, user := range users {
		avatarURL := "/assets/static/img/ui/placeholder_user.png"
		if user.Avatar != nil && user.Avatar.Link != "" {
			avatarURL = user.Avatar.Link
		}

		accounts = append(accounts, map[string]any{
			"id":         user.ID,
			"username":   user.Username,
			"email":      user.Email,
			"state":      user.State,
			"created_at": user.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"avatar_url": avatarURL,
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", accounts)
}

func (a *APIv1) BanUser(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	userID := c.Params("id")
	if userID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err == gorm.ErrRecordNotFound || user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	state := user.State
	state.Set(constants.USER_IS_BANNED)

	if err := a.ParentServer.Accounts.DB.UpdateUserState(userID, state); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to ban user.", nil)
	}

	return APIResult(c, fiber.StatusOK, "User banned successfully.", nil)
}

func (a *APIv1) UnbanUser(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	userID := c.Params("id")
	if userID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err == gorm.ErrRecordNotFound || user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	state := user.State
	state.Clear(constants.USER_IS_BANNED)

	if err := a.ParentServer.Accounts.DB.UpdateUserState(userID, state); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to unban user.", nil)
	}

	return APIResult(c, fiber.StatusOK, "User unbanned successfully.", nil)
}

func (a *APIv1) AdminDeleteAccount(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	userID := c.Params("id")
	if userID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err == gorm.ErrRecordNotFound || user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}
	if user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}

	tx := a.ParentServer.DB.DB.Begin()

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserGameSave{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete game saves.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserEvent{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete events.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserSession{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete sessions.", nil)
	}

	if err := tx.Where("user_id = ? OR friend_id = ?", userID, userID).Delete(&types.Friend{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete friends.", nil)
	}

	if err := tx.Where("sender_id = ? OR receiver_id = ?", userID, userID).Delete(&types.FriendRequest{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete friend requests.", nil)
	}

	if err := tx.Where("user_id = ? OR blocked_id = ?", userID, userID).Delete(&types.Blocklist{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete blocklist entries.", nil)
	}

	if err := tx.Where("sender_id = ? OR receiver_id = ?", userID, userID).Delete(&types.Message{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete messages.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.Notification{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete notifications.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserPlayedGame{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete played games.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.Achievement{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete achievements.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.GameComment{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete comments.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.DeveloperMember{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete developer memberships.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserGoogle{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete Google account.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserDiscord{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete Discord account.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserGitHub{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete GitHub account.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserTOTP{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete TOTP settings.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.Verification{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete verifications.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.EmailChangeToken{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete email change tokens.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.RecoveryCode{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete recovery codes.", nil)
	}

	if err := tx.Where("user_id = ?", userID).Delete(&types.UserReport{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete reports.", nil)
	}

	if err := tx.Delete(user).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete user.", nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to commit transaction.", nil)
	}

	// Log the event against the acting admin: the deleted user's own event rows
	// are removed with the account, and inserting one for a deleted user would
	// violate the user_events foreign key.
	if claims != nil && claims.ULID != userID {
		common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
			UserID:     claims.ULID,
			EventID:    "user_deleted",
			Details:    "Deleted user " + user.Username + " (" + userID + ")",
			Successful: true,
		})
	}

	return APIResult(c, fiber.StatusOK, "Account deleted successfully.", nil)
}

func (a *APIv1) GetAdminReports(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	reportType := c.Query("type", "")

	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 50
	}
	if page < 1 {
		page = 1
	}

	offset := (page - 1) * limit

	var reports []*types.UserReport
	query := a.Database.DB.Preload("User").Preload("SubmittedUser").Preload("ReportTag").Order("created_at DESC").Limit(limit).Offset(offset)

	if reportType != "" {
		query = query.Where("report_tag_id = ?", reportType)
	}

	if err := query.Find(&reports).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var reportList = []map[string]any{}
	for _, report := range reports {
		reportTag := ""
		if report.ReportTag != nil {
			reportTag = report.ReportTag.ID
		}

		reporterName := ""
		if report.User != nil {
			reporterName = report.User.Username
		}
		reportedName := ""
		if report.SubmittedUser != nil {
			reportedName = report.SubmittedUser.Username
		}

		reportList = append(reportList, map[string]any{
			"id":                  report.ID,
			"user_id":             report.UserID,
			"reporter_username":   reporterName,
			"reported_username":   reportedName,
			"submitted_user_id":   report.SubmittedUserID,
			"report_type":         reportTag,
			"details":             report.Details,
			"created_at":          report.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", reportList)
}

func (a *APIv1) ResolveReport(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	reportID := c.Params("id")
	if reportID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Report ID is required.", nil)
	}

	var args struct {
		Action string `json:"action"`
	}

	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	var report types.UserReport
	if err := a.Database.DB.First(&report, "id = ?", reportID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Report not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve report.", nil)
	}

	switch args.Action {
	case "dismiss":
		_ = a.Database.DB.Delete(&report)
	case "warn":
		if err := a.setUserStateFlag(report.SubmittedUserID, constants.USER_IS_WARNED, true); err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to warn user.", nil)
		}
		_ = a.Database.DB.Delete(&report)
	case "ban":
		if err := a.setUserStateFlag(report.SubmittedUserID, constants.USER_IS_BANNED, true); err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to ban user.", nil)
		}
		_ = a.Database.DB.Delete(&report)
	default:
		return APIResult(c, fiber.StatusBadRequest, "Invalid action.", nil)
	}

	return APIResult(c, fiber.StatusOK, "Report resolved successfully.", nil)
}

// setUserStateFlag changes a single flag of a user's state while preserving all other flags.
func (a *APIv1) setUserStateFlag(userID string, flag uint, enabled bool) error {
	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return gorm.ErrRecordNotFound
	}

	state := user.State
	if enabled {
		state.Set(flag)
	} else {
		state.Clear(flag)
	}

	return a.ParentServer.Accounts.DB.UpdateUserState(userID, state)
}

func (a *APIv1) CreateReport(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Please login first.", nil)
	}

	var args struct {
		SubmittedUserID string `json:"submitted_user_id"`
		ReportTagID     string `json:"report_tag_id"`
		Details         string `json:"details"`
	}

	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	if args.SubmittedUserID == "" || args.ReportTagID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Submitted user and report type are required.", nil)
	}

	submittedUserID := args.SubmittedUserID
	if len(submittedUserID) != 26 {
		var targetUser *types.User
		if err := a.Database.DB.Where("username = ?", submittedUserID).First(&targetUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return APIResult(c, fiber.StatusNotFound, "Reported user not found.", nil)
			}
			return APIResult(c, fiber.StatusInternalServerError, "Failed to lookup reported user.", nil)
		}
		submittedUserID = targetUser.ID
	}

	reportTagID := args.ReportTagID
	switch args.ReportTagID {
	case "user", "developer":
		reportTagID = "bullying"
	case "game":
		reportTagID = "nsfw"
	}

	report := types.UserReport{
		ID:              ulid.Make().String(),
		UserID:          claims.ULID,
		SubmittedUserID: submittedUserID,
		ReportTagID:     &reportTagID,
		Details:         args.Details,
		CreatedAt:       time.Now(),
	}

	if err := a.Database.DB.Create(&report).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to create report.", nil)
	}

	return APIResult(c, fiber.StatusOK, "Report submitted successfully.", nil)
}

func (a *APIv1) GetAdminStorage(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	var totalGames int64
	a.Database.DB.Model(&types.DeveloperGame{}).Count(&totalGames)

	var totalSaves int64
	a.Database.DB.Model(&types.UserGameSave{}).Count(&totalSaves)

	var totalImages int64
	a.Database.DB.Model(&types.Image{}).Count(&totalImages)

	data := map[string]any{
		"total_hosted_games": totalGames,
		"total_cloud_saves": totalSaves,
		"total_images":      totalImages,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

func (a *APIv1) GetAdminSettings(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	data := map[string]any{
		"admin_email": a.ParentServer.AdminEmail,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

func (a *APIv1) UpdateAdminSettings(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	var args struct {
		AdminEmail string `json:"admin_email" form:"admin_email"`
	}

	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	if args.AdminEmail != "" {
		a.ParentServer.AdminEmail = args.AdminEmail
	}

	data := map[string]any{
		"admin_email": a.ParentServer.AdminEmail,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

// GetAdminGames returns game submissions. By default it returns only pending
// games; pass ?all=1 to return every game for management.
func (a *APIv1) GetAdminGames(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	type game_entry struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		DeveloperID string `json:"developer_id"`
		Developer   string `json:"developer"`
		CreatedAt   string `json:"created_at"`
		State       uint8  `json:"state"`
	}

	showAll := c.Query("all") == "1"

	var games []*types.DeveloperGame
	if showAll {
		if err := a.Database.DB.Preload("Developer").Order("created_at DESC").Find(&games).Error; err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve games.", nil)
		}
	} else {
		games = a.Database.GetPendingGames()
	}

	entries := []*game_entry{}
	for _, game := range games {
		developer := ""
		if game.Developer != nil {
			developer = game.Developer.Name
		}
		entries = append(entries, &game_entry{
			ID:          game.ID,
			Name:        game.Name,
			Description: game.Description,
			DeveloperID: game.DeveloperID,
			Developer:   developer,
			CreatedAt:   game.CreatedAt.Format(time.RFC3339),
			State:       uint8(game.State),
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", entries)
}

// ApproveGame publishes a pending submission by moving it into projects_public
// and marking it active and verified.
func (a *APIv1) ApproveGame(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	game, err := a.getReviewableGame(c)
	if err != nil {
		return err
	}

	private_dir := filepath.Join(a.ParentServer.HostedPath, "projects_private", game.ID)
	public_dir := filepath.Join(a.ParentServer.HostedPath, "projects_public", game.ID)

	source_dir := private_dir
	if _, err := os.Stat(filepath.Join(public_dir, "index.html")); err == nil {
		source_dir = public_dir
	} else if _, err := os.Stat(filepath.Join(private_dir, "index.html")); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "The uploaded game package is missing.", nil)
	}

	if source_dir != public_dir {
		if err := os.MkdirAll(filepath.Dir(public_dir), 0755); err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to publish the game package.", nil)
		}
		os.RemoveAll(public_dir)
		if err := os.Rename(source_dir, public_dir); err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to publish the game package.", nil)
		}
	}

	state := game.State
	state.ManySet(constants.GAME_IS_ACTIVE, constants.GAME_IS_VERIFIED)
	state.Clear(constants.GAME_WAS_REJECTED)

	if err := a.Database.DB.Model(&types.DeveloperGame{}).Where("id = ?", game.ID).Update("state", state).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to publish the game.", nil)
	}

	// Public game listings are cached, so drop them to make the game visible.
	a.Database.Cache.Flush()

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "game_approved",
		Details:    "Approved game " + game.Name + " (" + game.ID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Game approved successfully.", nil)
}

// RejectGame discards a pending submission while keeping the record so the
// developer can see that it was reviewed.
func (a *APIv1) RejectGame(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	game, err := a.getReviewableGame(c)
	if err != nil {
		return err
	}

	// Take down the staged package and the optional project source.
	if err := os.RemoveAll(filepath.Join(a.ParentServer.HostedPath, "projects_private", game.ID)); err != nil {
		log.Errorf("Failed to remove the staged package for %s: %v", game.ID, err)
	}
	os.Remove(filepath.Join(a.ParentServer.HostedPath, "projects_source", game.ID+".sb3"))

	state := game.State
	state.Set(constants.GAME_WAS_REJECTED)

	if err := a.Database.DB.Model(&types.DeveloperGame{}).Where("id = ?", game.ID).Update("state", state).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to reject the game.", nil)
	}

	a.Database.Cache.Flush()

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "game_rejected",
		Details:    "Rejected game " + game.Name + " (" + game.ID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Game rejected successfully.", nil)
}

// getReviewableGame loads the game referenced by the request and rejects any
// submission that has already been published.
func (a *APIv1) getReviewableGame(c *fiber.Ctx) (*types.DeveloperGame, error) {
	game_id := c.Params("id")
	if game_id == "" {
		return nil, APIResult(c, fiber.StatusBadRequest, "Game ID is required.", nil)
	}

	var game types.DeveloperGame
	if err := a.Database.DB.Preload("Developer").First(&game, "id = ?", game_id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, APIResult(c, fiber.StatusNotFound, "Game not found.", nil)
		}
		return nil, APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve the game.", nil)
	}

	if game.State.Read(constants.GAME_IS_ACTIVE) && game.State.Read(constants.GAME_IS_VERIFIED) {
		return nil, APIResult(c, fiber.StatusBadRequest, "Game has already been published.", nil)
	}

	return &game, nil
}

func (a *APIv1) AdminDeleteGame(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	gameID := c.Params("id")
	if gameID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Game ID is required.", nil)
	}

	var game types.DeveloperGame
	if err := a.Database.DB.First(&game, "id = ?", gameID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Game not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve the game.", nil)
	}

	_ = os.RemoveAll(filepath.Join(a.ParentServer.HostedPath, "projects_private", gameID))
	_ = os.RemoveAll(filepath.Join(a.ParentServer.HostedPath, "projects_public", gameID))
	_ = os.Remove(filepath.Join(a.ParentServer.HostedPath, "projects_source", gameID+".sb3"))

	_ = a.Database.DB.Exec("DELETE FROM developer_game_features WHERE developer_game_id = ?", gameID)

	if err := a.Database.DB.Delete(&game).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete the game.", nil)
	}

	a.Database.Cache.Flush()

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     a.ParentServer.Authorization.GetNormalClaims(c).ULID,
		EventID:    "game_deleted",
		Details:    "Deleted game " + game.Name + " (" + game.ID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Game deleted successfully.", nil)
}

func (a *APIv1) AdminUpdateGameVisibility(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	gameID := c.Params("id")
	if gameID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Game ID is required.", nil)
	}

	var args struct {
		Visibility string `json:"visibility"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	visibility := strings.ToLower(strings.TrimSpace(args.Visibility))
	if visibility != "private" && visibility != "public" {
		return APIResult(c, fiber.StatusBadRequest, "Visibility must be 'private' or 'public'.", nil)
	}

	var game types.DeveloperGame
	if err := a.Database.DB.First(&game, "id = ?", gameID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Game not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve the game.", nil)
	}

	isActive := game.State.Read(constants.GAME_IS_ACTIVE)
	isVerified := game.State.Read(constants.GAME_IS_VERIFIED)
	isPrivate := isActive && !isVerified

	if visibility == "private" && !isPrivate {
		publicDir := filepath.Join(a.ParentServer.HostedPath, "projects_public", game.ID)
		privateDir := filepath.Join(a.ParentServer.HostedPath, "projects_private", game.ID)

		_ = os.MkdirAll(filepath.Dir(privateDir), 0755)
		_ = os.Rename(publicDir, privateDir)

		state := game.State
		state.Clear(constants.GAME_IS_VERIFIED)

		if err := a.Database.DB.Model(&types.DeveloperGame{}).Where("id = ?", game.ID).Update("state", state).Error; err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to update game visibility.", nil)
		}

		a.Database.Cache.Flush()
		return APIResult(c, fiber.StatusOK, "Game is now private.", nil)
	}

	if visibility == "public" && isPrivate {
		privateDir := filepath.Join(a.ParentServer.HostedPath, "projects_private", game.ID)
		publicDir := filepath.Join(a.ParentServer.HostedPath, "projects_public", game.ID)

		sourceDir := privateDir
		if _, err := os.Stat(filepath.Join(publicDir, "index.html")); err == nil {
			sourceDir = publicDir
		} else if _, err := os.Stat(filepath.Join(privateDir, "index.html")); err != nil {
			return APIResult(c, fiber.StatusBadRequest, "The uploaded game package is missing.", nil)
		}

		if sourceDir != publicDir {
			_ = os.MkdirAll(filepath.Dir(publicDir), 0755)
			_ = os.RemoveAll(publicDir)
			if err := os.Rename(sourceDir, publicDir); err != nil {
				return APIResult(c, fiber.StatusInternalServerError, "Failed to publish the game package.", nil)
			}
		}

		state := game.State
		state.ManySet(constants.GAME_IS_ACTIVE, constants.GAME_IS_VERIFIED)
		state.Clear(constants.GAME_WAS_REJECTED)

		if err := a.Database.DB.Model(&types.DeveloperGame{}).Where("id = ?", game.ID).Update("state", state).Error; err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to update game visibility.", nil)
		}

		a.Database.Cache.Flush()
		return APIResult(c, fiber.StatusOK, "Game is now public.", nil)
	}

	a.Database.Cache.Flush()
	return APIResult(c, fiber.StatusOK, "Game visibility unchanged.", nil)
}

func (a *APIv1) AdminReplaceGameFile(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	gameID := c.Params("id")
	if gameID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Game ID is required.", nil)
	}

	var game types.DeveloperGame
	if err := a.Database.DB.First(&game, "id = ?", gameID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Game not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve the game.", nil)
	}

	gameFile, err := c.FormFile("game")
	if err != nil {
		return APIResult(c, fiber.StatusBadRequest, "A game package (.zip) is required.", nil)
	}
	if !strings.HasSuffix(strings.ToLower(gameFile.Filename), ".zip") {
		return APIResult(c, fiber.StatusBadRequest, "The game package must be a .zip file.", nil)
	}
	if gameFile.Size > max_upload_size {
		return APIResult(c, fiber.StatusBadRequest, "The game package must be smaller than 64MB.", nil)
	}

	tmpFile, err := os.CreateTemp("", "clomega-game-*.zip")
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to store the uploaded game package.", nil)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := c.SaveFile(gameFile, tmpPath); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to store the uploaded game package.", nil)
	}

	targetDir := filepath.Join(a.ParentServer.HostedPath, "projects_public", game.ID)
	if game.State.Read(constants.GAME_IS_ACTIVE) && !game.State.Read(constants.GAME_IS_VERIFIED) {
		targetDir = filepath.Join(a.ParentServer.HostedPath, "projects_private", game.ID)
	}

	if err := extractGameArchive(tmpPath, targetDir); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid game package: "+err.Error(), nil)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "index.html")); err != nil {
		os.RemoveAll(targetDir)
		return APIResult(c, fiber.StatusBadRequest, "Invalid game package: index.html was not found in the archive.", nil)
	}

	a.Database.Cache.Flush()

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     a.ParentServer.Authorization.GetNormalClaims(c).ULID,
		EventID:    "game_file_replaced",
		Details:    "Replaced files for game " + game.Name + " (" + game.ID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Game file replaced successfully.", nil)
}

func (a *APIv1) AdminGetUserCloudSaves(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	userID := c.Params("id")
	if userID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err == gorm.ErrRecordNotFound || user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}

	type cloudSaveRow struct {
		UserID            string
		Username          string
		SaveSlot          uint8
		SaveData          string
		UpdatedAt         time.Time
		DeveloperGameID   string
		DeveloperGameName string
		GameDescription   string
		ThumbnailLink     string
	}

	var rows []cloudSaveRow
	err = a.Database.DB.Table("user_game_saves").
		Select("user_game_saves.user_id, users.username, user_game_saves.save_slot, user_game_saves.save_data, user_game_saves.updated_at, developer_games.id AS developer_game_id, developer_games.name AS developer_game_name, developer_games.description AS developer_game_description, images.link AS developer_game_thumbnail_link").
		Joins("LEFT JOIN developer_games ON developer_games.id = user_game_saves.developer_game_id").
		Joins("LEFT JOIN images ON images.id = developer_games.thumbnail_id").
		Joins("LEFT JOIN users ON users.id = user_game_saves.user_id").
		Where("user_game_saves.user_id = ?", userID).
		Order("user_game_saves.developer_game_id ASC, user_game_saves.save_slot ASC").
		Scan(&rows).Error

	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve cloud saves.", nil)
	}

	usersByID := map[string]*types.User{}
	for _, row := range rows {
		if _, ok := usersByID[row.UserID]; !ok {
			u, err := a.ParentServer.Accounts.DB.GetUser(row.UserID)
			if err == nil && u != nil {
				usersByID[row.UserID] = u
			}
		}
	}

	var saves = []map[string]any{}
	for _, row := range rows {
		saveData := row.SaveData
		if saveData == "" {
			saveData = "null"
		}

		if u, ok := usersByID[row.UserID]; ok && saveData != "null" {
			if decrypted, err := a.ParentServer.Accounts.DB.Decrypt(u, saveData); err == nil {
				saveData = decrypted
			}
		}

		saves = append(saves, map[string]any{
			"user_id": row.UserID,
			"username": row.Username,
			"save_slot": row.SaveSlot,
			"save_data": saveData,
			"updated_at": row.UpdatedAt,
			"developer_game_id": row.DeveloperGameID,
			"developer_game_name": row.DeveloperGameName,
			"game_description": row.GameDescription,
			"thumbnail_link": row.ThumbnailLink,
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", saves)
}

func (a *APIv1) AdminGetAllCloudSaves(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	type cloudSaveRow struct {
		UserID          string
		Username        string
		SaveSlot        uint8
		SaveData        string
		UpdatedAt       time.Time
		DeveloperGameID string
		DeveloperGameName string
		GameDescription string
		ThumbnailLink   string
	}

	var rows []cloudSaveRow
	err := a.Database.DB.Table("user_game_saves").
		Select("user_game_saves.user_id, users.username, user_game_saves.save_slot, user_game_saves.save_data, user_game_saves.updated_at, developer_games.id AS developer_game_id, developer_games.name AS developer_game_name, developer_games.description AS developer_game_description, images.link AS developer_game_thumbnail_link").
		Joins("LEFT JOIN developer_games ON developer_games.id = user_game_saves.developer_game_id").
		Joins("LEFT JOIN images ON images.id = developer_games.thumbnail_id").
		Joins("LEFT JOIN users ON users.id = user_game_saves.user_id").
		Order("user_game_saves.user_id ASC, user_game_saves.developer_game_id ASC, user_game_saves.save_slot ASC").
		Scan(&rows).Error

	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve cloud saves.", nil)
	}

	usersByID := map[string]*types.User{}
	for _, row := range rows {
		if _, ok := usersByID[row.UserID]; !ok {
			u, err := a.ParentServer.Accounts.DB.GetUser(row.UserID)
			if err == nil && u != nil {
				usersByID[row.UserID] = u
			}
		}
	}

	var saves = []map[string]any{}
	for _, row := range rows {
		saveData := row.SaveData
		if saveData == "" {
			saveData = "null"
		}

		if u, ok := usersByID[row.UserID]; ok && saveData != "null" {
			if decrypted, err := a.ParentServer.Accounts.DB.Decrypt(u, saveData); err == nil {
				saveData = decrypted
			}
		}

		saves = append(saves, map[string]any{
			"user_id": row.UserID,
			"username": row.Username,
			"save_slot": row.SaveSlot,
			"save_data": saveData,
			"updated_at": row.UpdatedAt,
			"developer_game_id": row.DeveloperGameID,
			"developer_game_name": row.DeveloperGameName,
			"game_description": row.GameDescription,
			"thumbnail_link": row.ThumbnailLink,
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", saves)
}

func (a *APIv1) AdminDeleteCloudSave(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	userID := c.Params("id")
	if userID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	slotStr := c.Params("slot")
	if slotStr == "" {
		return APIResult(c, fiber.StatusBadRequest, "Save slot is required.", nil)
	}

	var slot uint8
	if _, err := fmt.Sscanf(slotStr, "%d", &slot); err != nil || slot < 1 || slot > 10 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid save slot.", nil)
	}

	var args struct {
		DeveloperGameID string `json:"developer_game_id" form:"developer_game_id"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err == gorm.ErrRecordNotFound || user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}

	result := a.Database.DB.Where("user_id = ? AND save_slot = ? AND developer_game_id = ?", userID, slot, args.DeveloperGameID).Delete(&types.UserGameSave{})
	if result.Error != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to delete cloud save.", nil)
	}

	if result.RowsAffected == 0 {
		return APIResult(c, fiber.StatusNotFound, "Cloud save not found.", nil)
	}

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     a.ParentServer.Authorization.GetNormalClaims(c).ULID,
		EventID:    "admin_cloud_save_deleted",
		Details:    "Admin deleted cloud save for user " + user.Username + " (" + userID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Cloud save deleted successfully.", nil)
}

func (a *APIv1) AdminUpdateCloudSave(c *fiber.Ctx) error {
	if !a.ParentServer.IsAdmin(c) {
		return APIResult(c, fiber.StatusForbidden, "Forbidden.", nil)
	}

	userID := c.Params("id")
	if userID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	slotStr := c.Params("slot")
	if slotStr == "" {
		return APIResult(c, fiber.StatusBadRequest, "Save slot is required.", nil)
	}

	var slot uint8
	if _, err := fmt.Sscanf(slotStr, "%d", &slot); err != nil || slot < 1 || slot > 10 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid save slot.", nil)
	}

	var args struct {
		DeveloperGameID string `json:"developer_game_id" validate:"required"`
		SaveData        string `json:"save_data"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if args.DeveloperGameID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Developer game ID is required.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(userID)
	if err == gorm.ErrRecordNotFound || user == nil {
		return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
	}
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}

	var save types.UserGameSave
	result := a.Database.DB.First(&save, "user_id = ? AND save_slot = ? AND developer_game_id = ?", userID, slot, args.DeveloperGameID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Cloud save not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve cloud save.", nil)
	}

	if args.SaveData != "" {
		encrypted, err := a.ParentServer.Accounts.DB.Encrypt(user, args.SaveData)
		if err != nil {
			return APIResult(c, fiber.StatusInternalServerError, "Failed to encrypt save data.", nil)
		}
		save.SaveData = encrypted
	}

	if err := a.Database.DB.Save(&save).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to update cloud save.", nil)
	}

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     a.ParentServer.Authorization.GetNormalClaims(c).ULID,
		EventID:    "admin_cloud_save_updated",
		Details:    "Admin updated cloud save for user " + user.Username + " (" + userID + ")",
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "Cloud save updated successfully.", nil)
}

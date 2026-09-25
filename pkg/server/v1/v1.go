package v1

import (
	"time"

	account_structs "github.com/cloudlink-omega/accounts/pkg/structs"
	"github.com/cloudlink-omega/backend/pkg/database"
	"github.com/cloudlink-omega/backend/pkg/structs"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

type APIv1 struct {
	App          *fiber.App
	ParentServer *structs.Server
	Database     *database.Database
	EmailConfig  *account_structs.MailConfig
}

type Result struct {
	Result string `json:"result"`
	Data   any    `json:"data,omitempty"`
}

func New(s *structs.Server) *APIv1 {
	api := &APIv1{
		App:          fiber.New(),
		ParentServer: s,
		Database:     s.DB,
		EmailConfig:  s.MailConfig,
	}

	// Cloud save slots feature
	api.App.Post("/save", api.Save)
	api.App.Post("/load", api.Load)
	api.App.Get("/cloud-saves", api.GetCloudSaves)
	api.App.Post("/cloud-saves/decrypt", api.CloudSaveDecrypt)
	api.App.Post("/cloud-saves/delete", api.DeleteCloudSave)

	// Friends feature
	api.App.Get("/friends/search", api.SearchUsers)
	api.App.Post("/friends/request", api.SendFriendRequest)
	api.App.Post("/friends/request/:id/accept", api.AcceptFriendRequest)
	api.App.Post("/friends/request/:id/reject", api.RejectFriendRequest)
	api.App.Delete("/friends/request/:id", api.CancelFriendRequest)
	api.App.Get("/friends", api.GetFriends)
	api.App.Get("/friends/requests", api.GetFriendRequests)
	api.App.Delete("/friends/:id", api.RemoveFriend)
	api.App.Get("/friends/:user_id/messages", api.GetMessages)
	api.App.Post("/friends/:user_id/messages", api.SendMessage)
	api.App.Post("/friends/block", api.BlockUser)
	api.App.Post("/friends/unblock", api.UnblockUser)
	api.App.Get("/friends/blocklist", api.GetBlocklist)
	api.App.Get("/notifications", api.GetNotifications)
	api.App.Post("/notifications/:id/read", api.MarkNotificationRead)
	api.App.Get("/notifications/unread-count", api.GetUnreadNotificationCount)
	api.App.Post("/notifications/read-all", api.MarkAllNotificationsRead)

	// Achievements API
	api.App.Get("/achievements", api.GetAchievements)
	api.App.Get("/achievements/:game_id", api.GetAchievementsByGame)
	api.App.Post("/achievements/trigger", api.TriggerAchievement)

	// Profile API
	api.App.Get("/profile", api.GetProfile)
	api.App.Put("/profile", api.UpdateProfile)
	api.App.Post("/profile/avatar", api.UploadAvatar)

	// Settings API
	api.App.Get("/settings", api.GetSettings)
	api.App.Put("/settings", api.UpdateSettings)
	api.App.Put("/settings/password", api.ChangePassword)
	api.App.Get("/settings/export", api.ExportAccountData)
	api.App.Delete("/settings/account", api.DeleteAccount)

	// Points API
	api.App.Get("/points", api.GetPoints)
	api.App.Post("/points/checkin", api.CheckIn)
	api.App.Get("/points/history", api.GetPointsHistory)
	api.App.Post("/points/purchase", api.CreatePointsPurchase)
	api.App.Post("/points/purchase/confirm", api.ConfirmPointsPurchase)
	api.App.Post("/points/purchase/stripe", api.CreateStripeCheckoutSession)
	api.App.Get("/stripe/config", api.GetStripeConfig)
	api.App.Post("/points/deduct", api.RequestPointsDeduction)
	api.App.Post("/points/deduct/confirm", api.ConfirmPointsDeduction)
	api.App.Post("/points/transfer", api.TransferPoints)
	api.App.Post("/points/payment/process", api.ProcessPointsPayment)
	api.App.Post("/webhooks/stripe", api.StripeWebhook)
	api.App.Post("/reports", api.CreateReport)

	// Developers API. Throttle to 5 requests per minute
	dev_group := fiber.New()

	dev_group.Use(limiter.New(limiter.Config{
		Max:        5,
		Expiration: time.Minute,
	}))

	// Register a new developer account
	dev_group.Post("/register", api.RegisterDeveloper)

	// Register a new game
	dev_group.Post("/newgame", api.RegisterGame)

	// Upload a packaged game for review
	dev_group.Post("/upload", api.UploadGame)

	// List the caller's developer accounts and their games
	dev_group.Get("/games", api.GetMyDeveloperGames)

	// Replace the feature tags of one of the caller's games
	dev_group.Put("/games/:id/features", api.UpdateGameFeatures)

	// Mount the developer group
	api.App.Mount("/developer", dev_group)

	// Index
	api.App.Get("/", api.Index)

	// Admin API
	adminApp := fiber.New()
	adminApp.Use(limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
	}))
	adminApp.Get("/overview", api.GetAdminOverview)
	adminApp.Get("/logs", api.GetAdminLogs)
	adminApp.Get("/accounts", api.GetAdminAccounts)
	adminApp.Post("/ban/:id", api.BanUser)
	adminApp.Post("/unban/:id", api.UnbanUser)
	adminApp.Post("/delete/:id", api.AdminDeleteAccount)
	adminApp.Get("/storage", api.GetAdminStorage)
	adminApp.Get("/settings", api.GetAdminSettings)
	adminApp.Put("/settings", api.UpdateAdminSettings)
	adminApp.Get("/reports", api.GetAdminReports)
	adminApp.Post("/reports/:id/resolve", api.ResolveReport)
	adminApp.Get("/games", api.GetAdminGames)
	adminApp.Post("/games/:id/approve", api.ApproveGame)
	adminApp.Post("/games/:id/reject", api.RejectGame)
	api.App.Mount("/admin", adminApp)

	api.App.Get("/admin/overview2", api.GetAdminOverview)
	api.App.Get("/hello", func(c *fiber.Ctx) error {
		return c.SendString("hello")
	})

	return api
}

func APIResult(c *fiber.Ctx, status int, result string, data any) error {
	c.Set("Content-Type", "application/json")
	c.SendStatus(status)
	message, _ := json.Marshal(&Result{Result: result, Data: data})
	return c.SendString(string(message))
}

package server

import (
	"log"
	"embed"
	"net/http"
	"time"

	"github.com/cloudlink-omega/accounts"
	account_structs "github.com/cloudlink-omega/accounts/pkg/structs"
	"github.com/cloudlink-omega/backend/pkg/database"
	v0 "github.com/cloudlink-omega/backend/pkg/server/v0"
	v1 "github.com/cloudlink-omega/backend/pkg/server/v1"
	"github.com/cloudlink-omega/backend/pkg/structs"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

//go:embed views/**
var embedded_templates embed.FS

//go:embed assets/**
var embedded_static embed.FS

// TODO: add fields for the frontend server
type Server structs.Server

// New creates a new Server instance.
//
// Server is a collection of API endpoints, pages, and other things that make up the
// backend service. It is meant to be used as the main entrypoint to the service.
//
// The created instance is returned.
func New(
	// Server Name is used for labeling the server. Format: [Country Code]-[Server Nickname]-[Designation].
	server_name string,

	// PrimaryWebsite is the URL of the primary website.
	server_url string,

	// Point to the folder where the hosted assets are.
	hosted_path string,

	// Database.
	db *gorm.DB,

	// Cache interface.
	cache *types.DBCache,

	// Accounts API
	accounts_api *accounts.Accounts,

	// Email Config
	email_config *account_structs.MailConfig,

	// Disable DB - This service is completely disabled since it's a frontend-facing service
	bypass_db bool,

	// Admin email - if set, this email address will have admin access
	admin_email string,

) *Server {
	srv := &Server{
		ServerName: server_name,
		ServerURL:  server_url,
		Accounts:   accounts_api,
		Policy:     bluemonday.UGCPolicy(),
		HostedPath: hosted_path,
		MailConfig: email_config,
		AdminEmail: admin_email,
	}

	if bypass_db {
		log.Println("DB bypass enabled, skipping frontend server initialization.")
		return srv
	}

	// Initialize DB
	frontend_db := &database.Database{DB: db, Cache: cache}
	srv.DB = frontend_db

	// Initialize template engine
	engine := html.NewFileSystem(http.FS(embedded_templates), ".html")

	// Initialize app
	srv.App = fiber.New(fiber.Config{Views: engine, ErrorHandler: srv.ErrorPage})

	// Custom middleware to check for expired sessions and destroy them
	srv.App.Use(func(c *fiber.Ctx) error {
		claims := srv.Authorization.GetNormalClaims(c)
		if claims == nil {
			return c.Next()
		}

		if claims.ExpiresAt.Before(time.Now()) {
			srv.Accounts.APIv1.ClearCookie(c)
			srv.Accounts.DB.DeleteSession(claims.ID)
			return c.Next()
		}

		return c.Next()
	})

	// Configure routes
	srv.App.Get("/admin", srv.Admin)
	srv.App.Get("/dashboard", srv.Dashboard)
	srv.App.Get("/dashboard/achievements", srv.DashboardAchievements)
	srv.App.Get("/dashboard/cloud-saves", srv.CloudSaves)
	srv.App.Get("/dashboard/settings", srv.Settings)
	srv.App.Get("/profile", srv.Profile)
	srv.App.Get("/security", srv.Security)
	srv.App.Get("/developer", srv.DeveloperDashboard)
	// srv.App.Get("/omegadash", srv.OmegaDash)
	srv.App.Get("/terms", srv.Terms)
	srv.App.Get("/modal", srv.Modal)
	srv.App.Get("/about", srv.About)
	srv.App.Get("/explore", srv.Explore)
	srv.App.Get("/play/:id?", srv.Play)
	srv.App.Get("/friends", srv.Friends)
	srv.App.Get("/friends/requests", srv.FriendRequests)
	srv.App.Get("/friends/search", srv.SearchFriends)
	srv.App.Get("/friends/blocklist", srv.Blocklist)
	srv.App.Get("/friends/notifications", srv.Notifications)
	srv.App.Get("/chat/:user_id", srv.Chat)
	srv.App.Get("/points/confirm/deduct/:token", srv.PointsConfirmDeduction)
	srv.App.Get("/points/confirm/purchase/:purchaseID", srv.PointsConfirmPurchase)
	srv.App.Get("/dashboard/points/success", srv.PaymentSuccess)
	srv.App.Get("/dashboard/points/cancel", srv.PaymentCancelled)
	srv.App.Post("/points/payment/process", srv.PointsProcessPayment)
	srv.App.Get("/reports/new", srv.ReportNew)
	srv.App.Get("/", srv.Index)

	// Configure API Routes
	apiv0 := v0.New((*structs.Server)(srv))
	apiv1 := v1.New((*structs.Server)(srv))
	srv.App.Mount("/api/v0", apiv0.App)
	srv.App.Mount("/api/v1", apiv1.App)

	srv.App.Get("/api/v1/admin/overview", func(c *fiber.Ctx) error { return apiv1.GetAdminOverview(c) })
	srv.App.Get("/api/v1/admin/logs", func(c *fiber.Ctx) error { return apiv1.GetAdminLogs(c) })
	srv.App.Get("/api/v1/admin/accounts", func(c *fiber.Ctx) error { return apiv1.GetAdminAccounts(c) })
	srv.App.Get("/api/v1/admin/storage", func(c *fiber.Ctx) error { return apiv1.GetAdminStorage(c) })
	srv.App.Get("/api/v1/admin/settings", func(c *fiber.Ctx) error { return apiv1.GetAdminSettings(c) })
	srv.App.Put("/api/v1/admin/settings", func(c *fiber.Ctx) error { return apiv1.UpdateAdminSettings(c) })

	// Initialize assets path
	srv.App.Use("/assets", filesystem.New(filesystem.Config{
		Root:       http.FS(embedded_static),
		PathPrefix: "assets",
		Browse:     true,
	}))

	// Initialize middleware
	srv.App.Use(recover.New())
	srv.App.Use(logger.New())

	// Return created instance
	return srv
}

package structs

import (
	"github.com/cloudlink-omega/accounts"
	"github.com/cloudlink-omega/accounts/pkg/authorization"
	account_structs "github.com/cloudlink-omega/accounts/pkg/structs"
	"github.com/cloudlink-omega/accounts/pkg/constants"
	"github.com/cloudlink-omega/backend/pkg/database"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/microcosm-cc/bluemonday"
)

type Server struct {
	ServerName    string
	ServerURL     string
	DB            *database.Database
	App           *fiber.App
	Authorization *authorization.Auth
	Accounts      *accounts.Accounts
	Cache         *types.DBCache
	Policy        *bluemonday.Policy
	HostedPath    string
	MailConfig    *account_structs.MailConfig
	AdminEmail    string
}

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

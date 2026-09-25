package v1

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudlink-omega/accounts/pkg/constants"
	"github.com/cloudlink-omega/accounts/pkg/email"
	account_structs "github.com/cloudlink-omega/accounts/pkg/structs"
	"github.com/cloudlink-omega/storage/pkg/common"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/oklog/ulid/v2"
)

const (
	// max_upload_size caps each uploaded file.
	max_upload_size = 64 * 1024 * 1024
	// max_unzip_size caps the total size of an extracted game package.
	max_unzip_size = 256 * 1024 * 1024
	// max_zip_entries caps the number of entries in a game package.
	max_zip_entries = 5000
)

type RegisterDevArgs struct {
	Name        string   `json:"name" form:"name"`
	Description string   `json:"description" form:"description"`
	Members     []string `json:"members" form:"members"`
}

type RegisterGameArgs struct {
	DeveloperID string   `json:"developerid" form:"developerid"`
	Name        string   `json:"name" form:"name"`
	Description string   `json:"description" form:"description"`
	Features    []string `json:"features" form:"features"`
}

/*
 * Registers a new developer account.
 * POST /api/v1/developer/register
 *
 * Body Types:
 * JSON, Multipart Form
 *
 * Content:
 *
 *	{
 *		"name": "My Developer Studio Name",
 *		"description": "This is my new developer studio!",
 *		"members": ["user ID"...]
 *	}
 *
 * Response: application/json
 * 200 OK
 *	{
 *		"result": "OK",
 *		"data": "(Developer ID)"
 *	}
 * or
 * 200 OK
 * {
 *		"result": "OK; warnings: [...]",
 *		"data": "(Developer ID)"
 *	}
 */
func (a *APIv1) RegisterDeveloper(c *fiber.Ctx) error {
	if !a.ParentServer.Authorization.ValidFromNormal(c) {
		return APIResult(c, fiber.StatusUnauthorized, "Not logged in!", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	// Check if a server admin exists
	if !a.Database.AdminExists() {
		return APIResult(c, fiber.StatusTeapot, "Can't create developer; no server admin to notify!", nil)
	}

	// Read form
	var args RegisterDevArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	// Check if developer exists
	if a.Database.CheckForDeveloperNameConflict(args.Name) {
		return APIResult(c, fiber.StatusBadRequest, "Developer already exists!", nil)
	}

	// Set owner
	owner, err := a.ParentServer.Authorization.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	owner_member := &types.DeveloperMember{UserID: owner.ID}
	owner_member.State.ManySet(
		constants.DEVMEMBER_IS_OWNER,
		constants.DEVMEMBER_IS_ACTIVE,
	)

	// Prepare members
	var warnings []string
	var members []*types.DeveloperMember
	members = append(members, owner_member)
	for _, member := range args.Members {
		user, err := a.ParentServer.Authorization.DB.GetUser(member)
		if err != nil {
			return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
		}
		if user == nil {
			warnings = append(warnings, fmt.Sprintf("%s not found", member))
			continue
		}
		member := &types.DeveloperMember{UserID: user.ID}
		member.State.Set(constants.DEVMEMBER_IS_ACTIVE)
		members = append(members, member)
	}

	// Create the developer account
	developer_id := ulid.Make().String()
	developer := &types.Developer{
		ID:               developer_id,
		Name:             args.Name,
		Description:      args.Description,
		DeveloperMembers: members,
	}
	developer.State.Set(constants.DEVELOPER_IS_ACTIVE)
	a.Database.DB.Create(&developer)

	// Notify server admin
	server_admin := a.Database.GetAdmin()
	email.SendPlainEmail((*account_structs.MailConfig)(a.EmailConfig), &account_structs.EmailArgs{
		Subject:  "New developer registration",
		To:       server_admin.Email,
		Nickname: a.EmailConfig.Username,
	}, fmt.Sprintf(`Hello %s,
	
	A new developer account has been registered and is awaiting approval.

	Owner: %s (ID: %s, Email: %s) 
	ID: %s
	Name: %s
	Description: %s
	
	Regards,
	- %s`, server_admin.Username, owner.Username, owner.ID, owner.Email, developer_id, developer.Name, developer.Description, a.ParentServer.ServerName))

	if len(warnings) > 0 {
		res := "OK; Warnings: " + strings.Join(warnings, ", ")
		return APIResult(c, fiber.StatusOK, res, developer_id)
	}
	return APIResult(c, fiber.StatusOK, "OK", developer_id)
}

/*
 * Registers a new game.
 * POST /api/v1/developer/newgame
 *
 * Body Types:
 * JSON, Multipart Form
 *
 * Content:
 *
 *	{
 *		"developerid": "(Developer ID)",
 *		"name": "My Game Name",
 *		"description": "This is my new game!",
 *		"features": ["feature 1", "feature 2", ...]
 *	}
 *
 * Response: application/json
 * 200 OK
 *	{
 *		"result": "OK",
 *		"data": "(Game ID)"
 *	}
 * or
 * 200 OK
 * {
 *		"result": "OK; warnings: [...]",
 *		"data": "(Game ID)"
 *	}
 */
func (a *APIv1) RegisterGame(c *fiber.Ctx) error {
	if !a.ParentServer.Authorization.ValidFromNormal(c) {
		return APIResult(c, fiber.StatusUnauthorized, "Not logged in!", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	// Check if a server admin exists
	if !a.Database.AdminExists() {
		return APIResult(c, fiber.StatusTeapot, "Can't create game; no server admin to notify!", nil)
	}

	// Read form
	var args RegisterGameArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	// Check if developer exists
	if !a.Database.DeveloperExists(args.DeveloperID) {
		return APIResult(c, fiber.StatusBadRequest, "Developer not found!", nil)
	}

	// Get developer
	developer := a.Database.GetDeveloper(args.DeveloperID)
	if developer == nil {
		return APIResult(c, fiber.StatusBadRequest, "Developer not found!", nil)
	}

	// Check if user is developer owner
	var found bool
	log.Debugf("Developer members: %s", developer.DeveloperMembers)
	for _, dev := range developer.DeveloperMembers {
		if dev.UserID == claims.ULID && dev.State.Read(constants.DEVMEMBER_IS_OWNER) {
			found = true
			break
		}
	}
	if !found {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	// Prepare features
	var warnings []string
	var features []*types.FeatureTag
	for _, feature := range args.Features {
		// Verify the feature flag exists
		if a.Database.DB.Where("ID = ?", feature).First(&types.FeatureTag{}).RowsAffected == 0 {
			warnings = append(warnings, fmt.Sprintf("Feature flag %s not found", feature))
			continue
		}
		features = append(features, &types.FeatureTag{ID: feature})
	}

	game_id := ulid.Make().String()
	game := &types.DeveloperGame{
		ID:          game_id,
		DeveloperID: args.DeveloperID,
		Name:        args.Name,
		Description: args.Description,
		Features:    features,
	}

	// Create the game
	a.Database.DB.Create(&game)

	// Notify server admin
	server_admin := a.Database.GetAdmin()
	email.SendPlainEmail((*account_structs.MailConfig)(a.EmailConfig), &account_structs.EmailArgs{
		Subject:  "New developer game",
		To:       server_admin.Email,
		Nickname: a.EmailConfig.Username,
	}, fmt.Sprintf(`Hello %s,
	
	A developer has created a new game. It will not be visible until it is approved.

	Submitted by: %s (ID: %s, Email: %s) 
	Developer: %s (%s)
	ID: %s
	Name: %s
	Description: %s
	
	Regards,
	- %s`, server_admin.Username, claims.Username, claims.ULID, claims.Email, developer.Name, developer.ID, game_id, args.Name, args.Description, a.ParentServer.ServerName))

	if len(warnings) > 0 {
		res := "OK; Warnings: " + strings.Join(warnings, ", ")
		return APIResult(c, fiber.StatusOK, res, game_id)
	}
	return APIResult(c, fiber.StatusOK, "OK", game_id)
}

// isDeveloperMember reports whether the user may publish under the developer account.
// Both the owner and active members are allowed to submit games.
func isDeveloperMember(developer *types.Developer, user_id string) bool {
	for _, member := range developer.DeveloperMembers {
		if member.UserID != user_id {
			continue
		}
		if member.State.Read(constants.DEVMEMBER_IS_OWNER) || member.State.Read(constants.DEVMEMBER_IS_ACTIVE) {
			return true
		}
	}
	return false
}

// resolveFeatureTags keeps only the feature tags the server knows about and
// loads them as complete records, so the seeded descriptions are never cleared.
func resolveFeatureTags(a *APIv1, ids []string) []*types.FeatureTag {
	var tags []*types.FeatureTag
	seen := make(map[string]bool)

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		if _, known := types.GameFeatureTags[id]; !known {
			continue
		}

		var tag types.FeatureTag
		if err := a.Database.DB.Where("ID = ?", id).First(&tag).Error; err != nil {
			continue
		}

		seen[id] = true
		tags = append(tags, &tag)
	}

	return tags
}

/*
 * Uploads a packaged game for review.
 * POST /api/v1/developer/upload
 *
 * Body Types: Multipart Form
 *
 * Content:
 *
 *	"developerid": "(Developer ID)",
 *	"name": "My Game Name",
 *	"description": "This is my new game!",
 *	"features": ["(feature tag ID)", ...] (repeatable)
 *	"game": (zip archive exported by the TurboWarp Packager)
 *	"source": (optional .sb3 project file)
 *
 * The archive is extracted into projects_private and only moved into
 * projects_public once an admin approves the submission.
 *
 * Response: application/json
 * 200 OK
 *	{
 *		"result": "OK",
 *		"data": "(Game ID)"
 *	}
 */
func (a *APIv1) UploadGame(c *fiber.Ctx) error {
	if !a.ParentServer.Authorization.ValidFromNormal(c) {
		return APIResult(c, fiber.StatusUnauthorized, "Not logged in!", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	developer_id := c.FormValue("developerid")
	name := strings.TrimSpace(c.FormValue("name"))
	description := strings.TrimSpace(c.FormValue("description"))

	if developer_id == "" {
		return APIResult(c, fiber.StatusBadRequest, "Developer ID is required.", nil)
	}
	if name == "" {
		return APIResult(c, fiber.StatusBadRequest, "Game name is required.", nil)
	}

	// The dashboard sends the feature tags as repeated form fields.
	var feature_ids []string
	if form, err := c.MultipartForm(); err == nil && form != nil {
		feature_ids = form.Value["features"]
	}

	developer := a.Database.GetDeveloper(developer_id)
	if developer == nil {
		return APIResult(c, fiber.StatusBadRequest, "Developer not found!", nil)
	}

	if !isDeveloperMember(developer, claims.ULID) {
		return APIResult(c, fiber.StatusForbidden, "You are not a member of this developer account.", nil)
	}

	// The packaged game is mandatory, the project source is not.
	game_file, err := c.FormFile("game")
	if err != nil {
		return APIResult(c, fiber.StatusBadRequest, "A game package (.zip) is required.", nil)
	}
	if !strings.HasSuffix(strings.ToLower(game_file.Filename), ".zip") {
		return APIResult(c, fiber.StatusBadRequest, "The game package must be a .zip file.", nil)
	}
	if game_file.Size > max_upload_size {
		return APIResult(c, fiber.StatusBadRequest, "The game package must be smaller than 64MB.", nil)
	}

	source_file, source_err := c.FormFile("source")
	if source_err == nil {
		if !strings.HasSuffix(strings.ToLower(source_file.Filename), ".sb3") {
			return APIResult(c, fiber.StatusBadRequest, "The project source must be a .sb3 file.", nil)
		}
		if source_file.Size > max_upload_size {
			return APIResult(c, fiber.StatusBadRequest, "The project source must be smaller than 64MB.", nil)
		}
	}

	game_id := ulid.Make().String()
	dest_dir := filepath.Join(a.ParentServer.HostedPath, "projects_private", game_id)

	// Stage the archive outside of the game folder so it never gets served.
	tmp_file, err := os.CreateTemp("", "clomega-game-*.zip")
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to store the uploaded game package.", nil)
	}
	tmp_path := tmp_file.Name()
	tmp_file.Close()
	defer os.Remove(tmp_path)

	if err := c.SaveFile(game_file, tmp_path); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to store the uploaded game package.", nil)
	}

	if err := extractGameArchive(tmp_path, dest_dir); err != nil {
		os.RemoveAll(dest_dir)
		return APIResult(c, fiber.StatusBadRequest, "Invalid game package: "+err.Error(), nil)
	}

	// A packaged game must boot straight from the root of its folder.
	if _, err := os.Stat(filepath.Join(dest_dir, "index.html")); err != nil {
		os.RemoveAll(dest_dir)
		return APIResult(c, fiber.StatusBadRequest, "Invalid game package: index.html was not found in the archive.", nil)
	}

	var source_path string
	if source_err == nil {
		source_dir := filepath.Join(a.ParentServer.HostedPath, "projects_source")
		if err := os.MkdirAll(source_dir, 0755); err != nil {
			os.RemoveAll(dest_dir)
			return APIResult(c, fiber.StatusInternalServerError, "Failed to store the project source.", nil)
		}
		source_path = filepath.Join(source_dir, game_id+".sb3")
		if err := c.SaveFile(source_file, source_path); err != nil {
			os.RemoveAll(dest_dir)
			return APIResult(c, fiber.StatusInternalServerError, "Failed to store the project source.", nil)
		}
	}

	game := &types.DeveloperGame{
		ID:          game_id,
		DeveloperID: developer.ID,
		Name:        name,
		Description: description,
		Features:    resolveFeatureTags(a, feature_ids),
	}

	// The game starts out unverified, which keeps it hidden until it is approved.
	if err := a.Database.DB.Create(game).Error; err != nil {
		os.RemoveAll(dest_dir)
		if source_path != "" {
			os.Remove(source_path)
		}
		return APIResult(c, fiber.StatusInternalServerError, "Failed to create the game record.", nil)
	}

	// Notify server admin
	server_admin := a.Database.GetAdmin()
	if server_admin != nil {
		email.SendPlainEmail((*account_structs.MailConfig)(a.EmailConfig), &account_structs.EmailArgs{
			Subject:  "New game submission",
			To:       server_admin.Email,
			Nickname: a.EmailConfig.Username,
		}, fmt.Sprintf(`Hello %s,

	A developer has submitted a new game. It will not be visible until it is approved.

	Submitted by: %s (ID: %s, Email: %s)
	Developer: %s (%s)
	ID: %s
	Name: %s
	Description: %s

	Regards,
	- %s`, server_admin.Username, claims.Username, claims.ULID, claims.Email, developer.Name, developer.ID, game_id, name, description, a.ParentServer.ServerName))
	}

	common.LogEvent(a.ParentServer.DB.DB, &types.UserEvent{
		UserID:     claims.ULID,
		EventID:    "game_submitted",
		Details:    "Submitted game " + name + " (" + game_id + ") for developer " + developer.Name,
		Successful: true,
	})

	return APIResult(c, fiber.StatusOK, "OK", game_id)
}

/*
 * Replaces the feature tags of a game.
 * PUT /api/v1/developer/games/:id/features
 *
 * Body Types: JSON
 *
 * Content:
 *
 *	{
 *		"features": ["(feature tag ID)"...]
 *	}
 *
 * Response: application/json
 * 200 OK
 *	{
 *		"result": "OK"
 *	}
 */
func (a *APIv1) UpdateGameFeatures(c *fiber.Ctx) error {
	if !a.ParentServer.Authorization.ValidFromNormal(c) {
		return APIResult(c, fiber.StatusUnauthorized, "Not logged in!", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	var args struct {
		Features []string `json:"features"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	game := a.Database.GetGame(c.Params("id"))
	if game == nil {
		return APIResult(c, fiber.StatusNotFound, "Game not found!", nil)
	}

	developer := a.Database.GetDeveloper(game.DeveloperID)
	if developer == nil || !isDeveloperMember(developer, claims.ULID) {
		return APIResult(c, fiber.StatusForbidden, "You are not a member of this developer account.", nil)
	}

	if err := a.Database.DB.Model(game).Association("Features").Replace(resolveFeatureTags(a, args.Features)); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to update the game features.", nil)
	}

	// The game and explore listings are cached, so drop them to publish the change.
	a.Database.Cache.Flush()

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

/*
 * Lists the developer accounts the caller belongs to, along with their games.
 * GET /api/v1/developer/games
 *
 * Response: application/json
 * 200 OK
 *	{
 *		"result": "OK",
 *		"data": [ ...developers with their games... ]
 *	}
 */
func (a *APIv1) GetMyDeveloperGames(c *fiber.Ctx) error {
	if !a.ParentServer.Authorization.ValidFromNormal(c) {
		return APIResult(c, fiber.StatusUnauthorized, "Not logged in!", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)

	type game_entry struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		DeveloperID string   `json:"developer_id"`
		Status      string   `json:"status"`
		CreatedAt   string   `json:"created_at"`
		Features    []string `json:"features"`
	}

	type developer_entry struct {
		ID          string        `json:"id"`
		Name        string        `json:"name"`
		Description string        `json:"description"`
		IsOwner     bool          `json:"is_owner"`
		Games       []*game_entry `json:"games"`
	}

	entries := []*developer_entry{}

	for _, developer := range a.Database.GetDevelopersForUser(claims.ULID) {
		entry := &developer_entry{
			ID:          developer.ID,
			Name:        developer.Name,
			Description: developer.Description,
			IsOwner:     isDeveloperOwner(developer, claims.ULID),
			Games:       []*game_entry{},
		}

		for _, game := range a.Database.GetGamesForDeveloper(developer.ID) {
			status := "pending"
			if game.State.Read(constants.GAME_WAS_REJECTED) {
				status = "rejected"
			} else if game.State.Read(constants.GAME_IS_ACTIVE) && game.State.Read(constants.GAME_IS_VERIFIED) {
				status = "published"
			}

			features := []string{}
			for _, feature := range game.Features {
				if feature != nil {
					features = append(features, feature.ID)
				}
			}

			entry.Games = append(entry.Games, &game_entry{
				ID:          game.ID,
				Name:        game.Name,
				Description: game.Description,
				DeveloperID: game.DeveloperID,
				Status:      status,
				CreatedAt:   game.CreatedAt.Format(time.RFC3339),
				Features:    features,
			})
		}

		entries = append(entries, entry)
	}

	return APIResult(c, fiber.StatusOK, "OK", entries)
}

// isDeveloperOwner reports whether the user owns the developer account.
func isDeveloperOwner(developer *types.Developer, user_id string) bool {
	for _, member := range developer.DeveloperMembers {
		if member.UserID == user_id && member.State.Read(constants.DEVMEMBER_IS_OWNER) {
			return true
		}
	}
	return false
}

// extractGameArchive extracts a packaged game into dest_dir. It rejects path
// traversal, enforces size limits, and strips a single wrapping directory if
// the archive was exported that way.
func extractGameArchive(archive_path string, dest_dir string) error {
	reader, err := zip.OpenReader(archive_path)
	if err != nil {
		return fmt.Errorf("the archive could not be read")
	}
	defer reader.Close()

	if len(reader.File) > max_zip_entries {
		return fmt.Errorf("the archive contains too many files")
	}

	prefix := archiveRootPrefix(reader.File)

	if err := os.MkdirAll(dest_dir, 0755); err != nil {
		return fmt.Errorf("the archive could not be extracted")
	}

	var total uint64
	for _, entry := range reader.File {
		// Some zip writers use backslashes as separators, so normalize before
		// stripping the wrapper directory and converting to OS separators.
		normalized := strings.ReplaceAll(entry.Name, "\\", "/")
		name := filepath.Clean(filepath.FromSlash(strings.TrimPrefix(normalized, prefix)))
		if name == "." || name == "" {
			continue
		}
		if name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(name) {
			return fmt.Errorf("the archive contains an invalid path")
		}

		target := filepath.Join(dest_dir, name)
		if !strings.HasPrefix(target, filepath.Clean(dest_dir)+string(os.PathSeparator)) {
			return fmt.Errorf("the archive contains an invalid path")
		}

		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("the archive could not be extracted")
			}
			continue
		}

		total += entry.UncompressedSize64
		if total > max_unzip_size {
			return fmt.Errorf("the archive is too large when extracted")
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("the archive could not be extracted")
		}
		if err := writeZipEntry(entry, target); err != nil {
			return fmt.Errorf("the archive could not be extracted")
		}
	}

	return nil
}

// archiveRootPrefix returns the wrapping directory shared by every entry, or an
// empty string when the archive already has its files at the root.
func archiveRootPrefix(entries []*zip.File) string {
	has_root_index := false
	roots := map[string]bool{}

	for _, entry := range entries {
		name := strings.Trim(strings.ReplaceAll(entry.Name, "\\", "/"), "/")
		if name == "" {
			continue
		}
		if strings.EqualFold(name, "index.html") {
			has_root_index = true
		}
		if idx := strings.Index(name, "/"); idx >= 0 {
			roots[name[:idx]] = true
		}
	}

	if has_root_index || len(roots) != 1 {
		return ""
	}

	for root := range roots {
		return root + "/"
	}
	return ""
}

func writeZipEntry(entry *zip.File, target string) error {
	src, err := entry.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

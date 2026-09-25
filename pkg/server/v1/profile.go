package v1

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

// UpdateProfile updates the user's profile information
func (a *APIv1) UpdateProfile(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var req struct {
		Name     *string `json:"name"`
		Bio      *string `json:"bio"`
		Location *string `json:"location"`
		Website  *string `json:"website"`
	}

	if err := c.BodyParser(&req); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if err := a.ParentServer.Accounts.DB.UpdateUserProfile(claims.ULID, req.Name, req.Bio, req.Location, req.Website); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "Profile updated successfully.", nil)
}

// UploadAvatar handles avatar image upload
func (a *APIv1) UploadAvatar(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	file, err := c.FormFile("avatar")
	if err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Avatar file is required.", nil)
	}

	if file.Size > 5*1024*1024 {
		return APIResult(c, fiber.StatusBadRequest, "Avatar file size must be less than 5MB.", nil)
	}

	allowedTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/gif":  true,
		"image/webp": true,
	}

	if !allowedTypes[file.Header.Get("Content-Type")] {
		return APIResult(c, fiber.StatusBadRequest, "Invalid file type. Only JPEG, PNG, GIF, and WebP are allowed.", nil)
	}

	uploadDir := filepath.Join(a.ParentServer.HostedPath, "avatars")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to create upload directory.", nil)
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	imageID := ulid.Make().String()
	filename := imageID + ext
	savePath := filepath.Join(uploadDir, filename)

	if err := c.SaveFile(file, savePath); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to save file.", nil)
	}

	imageLink := fmt.Sprintf("%s/hosted/avatars/%s", a.ParentServer.ServerURL, filename)

	image := &types.Image{
		ID:        imageID,
		Link:      imageLink,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := a.ParentServer.Accounts.DB.CreateImage(image); err != nil {
		os.Remove(savePath)
		return APIResult(c, fiber.StatusInternalServerError, "Failed to create image record.", nil)
	}

	if err := a.ParentServer.Accounts.DB.UpdateUserAvatar(claims.ULID, &imageID); err != nil {
		os.Remove(savePath)
		a.ParentServer.Accounts.DB.DB.Delete(image)
		return APIResult(c, fiber.StatusInternalServerError, "Failed to update avatar.", nil)
	}

	return APIResult(c, fiber.StatusOK, "Avatar uploaded successfully.", fiber.Map{
		"avatar_id": imageID,
		"avatar_url": imageLink,
	})
}

// GetProfile returns the current user's profile
func (a *APIv1) GetProfile(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve profile.", nil)
	}

	response := fiber.Map{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
		"name":     user.Name,
		"bio":      user.Bio,
		"location": user.Location,
		"website":  user.Website,
	}

	if user.Avatar != nil {
		response["avatar"] = fiber.Map{
			"id":   user.Avatar.ID,
			"link": user.Avatar.Link,
		}
	} else {
		response["avatar"] = nil
	}

	return APIResult(c, fiber.StatusOK, "OK", response)
}


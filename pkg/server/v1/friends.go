package v1

import (
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
)

func createNotification(db *gorm.DB, userID string, notificationType string, message string) error {
	notification := &types.Notification{
		ID:      ulid.Make().String(),
		UserID:  userID,
		Type:    notificationType,
		Message: message,
		Read:    false,
	}
	return db.Create(notification).Error
}

// SearchUsers searches for users by username
func (a *APIv1) SearchUsers(c *fiber.Ctx) error {
	query := c.Query("q", "")
	if len(query) < 1 {
		return APIResult(c, fiber.StatusBadRequest, "Query parameter 'q' is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var users []*types.User
	if err := a.Database.DB.Preload("Avatar").Where("username LIKE ? AND is_public = ?", query+"%", true).Limit(20).Find(&users).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type UserSummary struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}

	results := make([]*UserSummary, 0, len(users))
	for _, user := range users {
		if user.ID == claims.ULID {
			continue
		}
		avatarURL := "/assets/static/img/ui/placeholder_user.png"
		if user.Avatar != nil && user.Avatar.Link != "" {
			avatarURL = user.Avatar.Link
		}
		results = append(results, &UserSummary{
			ID:        user.ID,
			Name:      user.Name,
			Username:  user.Username,
			AvatarURL: avatarURL,
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", results)
}

// MarkAllNotificationsRead marks all notifications as read for the current user
func (a *APIv1) MarkAllNotificationsRead(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if err := a.Database.DB.Model(&types.Notification{}).Where("user_id = ? AND `read` = ?", claims.ULID, false).Update("read", true).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// GetNotifications returns notifications for the current user
func (a *APIv1) GetNotifications(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var notifications []*types.Notification
	if err := a.Database.DB.Where("user_id = ?", claims.ULID).Order("created_at DESC").Limit(50).Find(&notifications).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type NotificationSummary struct {
		ID        string `json:"id"`
		Type      string `json:"type"`
		Message   string `json:"message"`
		Read      bool   `json:"read"`
		CreatedAt string `json:"created_at"`
	}

	results := make([]*NotificationSummary, 0, len(notifications))
	for _, notification := range notifications {
		results = append(results, &NotificationSummary{
			ID:        notification.ID,
			Type:      notification.Type,
			Message:   notification.Message,
			Read:      notification.Read,
			CreatedAt: notification.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", results)
}

// MarkNotificationRead marks a notification as read
func (a *APIv1) MarkNotificationRead(c *fiber.Ctx) error {
	notificationID := c.Params("id")
	if notificationID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Notification ID is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if err := a.Database.DB.Model(&types.Notification{}).Where("id = ? AND user_id = ?", notificationID, claims.ULID).Update("read", true).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// GetUnreadNotificationCount returns the count of unread notifications
func (a *APIv1) GetUnreadNotificationCount(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var count int64
	if err := a.Database.DB.Model(&types.Notification{}).Where("user_id = ? AND `read` = ?", claims.ULID, false).Count(&count).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", map[string]any{"count": count})
}

// SendFriendRequest sends a friend request to another user
func (a *APIv1) SendFriendRequest(c *fiber.Ctx) error {
	var args struct {
		ReceiverID string `json:"receiver_id" validate:"required,ulid"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if claims.ULID == args.ReceiverID {
		return APIResult(c, fiber.StatusBadRequest, "Cannot send friend request to yourself.", nil)
	}

	var receiver *types.User
	if err := a.Database.DB.First(&receiver, "id = ?", args.ReceiverID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "User not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var existingRequest *types.FriendRequest
	err := a.Database.DB.Where("((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)) AND status = ?", claims.ULID, args.ReceiverID, args.ReceiverID, claims.ULID, "pending").First(&existingRequest).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	if err == gorm.ErrRecordNotFound {
		existingRequest = nil
	}
	if existingRequest != nil {
		return APIResult(c, fiber.StatusBadRequest, "Friend request already exists.", nil)
	}

	var existingFriendship *types.Friend
	err = a.Database.DB.Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", claims.ULID, args.ReceiverID, args.ReceiverID, claims.ULID).First(&existingFriendship).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	if err == gorm.ErrRecordNotFound {
		existingFriendship = nil
	}
	if existingFriendship != nil {
		return APIResult(c, fiber.StatusBadRequest, "You are already friends.", nil)
	}

	var blocklist *types.Blocklist
	err = a.Database.DB.First(&blocklist, "user_id = ? AND blocked_id = ?", args.ReceiverID, claims.ULID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	if err == gorm.ErrRecordNotFound {
		blocklist = nil
	}
	if blocklist != nil {
		return APIResult(c, fiber.StatusForbidden, "User has blocked you.", nil)
	}

	var myBlocklist *types.Blocklist
	err = a.Database.DB.First(&myBlocklist, "user_id = ? AND blocked_id = ?", claims.ULID, args.ReceiverID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	if err == gorm.ErrRecordNotFound {
		myBlocklist = nil
	}
	if myBlocklist != nil {
		return APIResult(c, fiber.StatusBadRequest, "You have blocked this user.", nil)
	}

	request := &types.FriendRequest{
		ID:         ulid.Make().String(),
		SenderID:   claims.ULID,
		ReceiverID: args.ReceiverID,
		Status:     "pending",
	}
	if err := a.Database.DB.Create(request).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// CancelFriendRequest cancels a sent friend request
func (a *APIv1) CancelFriendRequest(c *fiber.Ctx) error {
	requestID := c.Params("id")
	if requestID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Request ID is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var request *types.FriendRequest
	if err := a.Database.DB.First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Friend request not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if request.SenderID != claims.ULID {
		return APIResult(c, fiber.StatusForbidden, "You are not authorized to cancel this request.", nil)
	}

	if request.Status != "pending" {
		return APIResult(c, fiber.StatusBadRequest, "Request is not pending.", nil)
	}

	if err := a.Database.DB.Delete(&request).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// AcceptFriendRequest accepts a friend request
func (a *APIv1) AcceptFriendRequest(c *fiber.Ctx) error {
	requestID := c.Params("id")
	if requestID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Request ID is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var request *types.FriendRequest
	if err := a.Database.DB.First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Friend request not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if request.ReceiverID != claims.ULID {
		return APIResult(c, fiber.StatusForbidden, "You are not authorized to accept this request.", nil)
	}

	if request.Status != "pending" {
		return APIResult(c, fiber.StatusBadRequest, "Request is not pending.", nil)
	}

	tx := a.Database.DB.Begin()
	if err := tx.Model(&request).Updates(map[string]any{"status": "accepted"}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Create(&types.Friend{ID: ulid.Make().String(), UserID: request.SenderID, FriendID: request.ReceiverID}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	if err := tx.Create(&types.Friend{ID: ulid.Make().String(), UserID: request.ReceiverID, FriendID: request.SenderID}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := createNotification(a.Database.DB, request.SenderID, "friend_request_accepted", claims.Username+" accepted your friend request."); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// RejectFriendRequest rejects a friend request
func (a *APIv1) RejectFriendRequest(c *fiber.Ctx) error {
	requestID := c.Params("id")
	if requestID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Request ID is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var request *types.FriendRequest
	if err := a.Database.DB.First(&request, "id = ?", requestID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusNotFound, "Friend request not found.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if request.ReceiverID != claims.ULID {
		return APIResult(c, fiber.StatusForbidden, "You are not authorized to reject this request.", nil)
	}

	if request.Status != "pending" {
		return APIResult(c, fiber.StatusBadRequest, "Request is not pending.", nil)
	}

	if err := a.Database.DB.Model(&request).Updates(map[string]any{"status": "rejected"}).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := createNotification(a.Database.DB, request.SenderID, "friend_request_rejected", claims.Username+" rejected your friend request."); err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// GetFriends returns the list of friends for the current user
func (a *APIv1) GetFriends(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var friendships []*types.Friend
	if err := a.Database.DB.Preload("Friend.Avatar").Where("user_id = ?", claims.ULID).Find(&friendships).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type FriendSummary struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}

	results := make([]*FriendSummary, 0, len(friendships))
	for _, friendship := range friendships {
		if friendship.Friend != nil {
			avatarURL := "/assets/static/img/ui/placeholder_user.png"
			if friendship.Friend.Avatar != nil && friendship.Friend.Avatar.Link != "" {
				avatarURL = friendship.Friend.Avatar.Link
			}
			results = append(results, &FriendSummary{
				ID:        friendship.Friend.ID,
				Name:      friendship.Friend.Name,
				Username:  friendship.Friend.Username,
				AvatarURL: avatarURL,
			})
		}
	}

	return APIResult(c, fiber.StatusOK, "OK", results)
}

// GetFriendRequests returns pending friend requests for the current user
func (a *APIv1) GetFriendRequests(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var requests []*types.FriendRequest
	if err := a.Database.DB.Preload("Sender.Avatar").Preload("Receiver.Avatar").Where("(receiver_id = ? AND status = ?) OR (sender_id = ? AND status = ?)", claims.ULID, "pending", claims.ULID, "pending").Find(&requests).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type RequestSummary struct {
		ID             string `json:"id"`
		SenderID       string `json:"sender_id"`
		ReceiverID     string `json:"receiver_id"`
		Sender         string `json:"sender"`
		SenderName     string `json:"sender_name"`
		Receiver       string `json:"receiver"`
		ReceiverName   string `json:"receiver_name"`
		SenderAvatar   string `json:"sender_avatar"`
		ReceiverAvatar string `json:"receiver_avatar"`
		Type           string `json:"type"`
		CreatedAt      string `json:"created_at"`
	}

	results := make([]*RequestSummary, 0, len(requests))
	for _, request := range requests {
		senderName := ""
		senderAvatar := "/assets/static/img/ui/placeholder_user.png"
		if request.Sender != nil {
			if request.Sender.Name != "" {
				senderName = request.Sender.Name
			} else {
				senderName = request.Sender.Username
			}
			if request.Sender.Avatar != nil && request.Sender.Avatar.Link != "" {
				senderAvatar = request.Sender.Avatar.Link
			}
		}
		receiverName := ""
	receiverAvatar := "/assets/static/img/ui/placeholder_user.png"
	if request.Receiver != nil {
		if request.Receiver.Name != "" {
			receiverName = request.Receiver.Name
		} else {
			receiverName = request.Receiver.Username
		}
		if request.Receiver.Avatar != nil && request.Receiver.Avatar.Link != "" {
			receiverAvatar = request.Receiver.Avatar.Link
		}
	}
	requestType := "received"
	if request.SenderID == claims.ULID {
		requestType = "sent"
	}
	results = append(results, &RequestSummary{
		ID:             request.ID,
		SenderID:       request.SenderID,
		ReceiverID:     request.ReceiverID,
		Sender:         request.Sender.Username,
		SenderName:     senderName,
		Receiver:       request.Receiver.Username,
		ReceiverName:   receiverName,
		SenderAvatar:   senderAvatar,
		ReceiverAvatar: receiverAvatar,
		Type:           requestType,
		CreatedAt:      request.CreatedAt.Format("2006-01-02 15:04:05"),
	})
	}

	return APIResult(c, fiber.StatusOK, "OK", results)
}

// GetMessages returns chat history between current user and another user
func (a *APIv1) GetMessages(c *fiber.Ctx) error {
	otherUserID := c.Params("user_id")
	if otherUserID == "" {
		return APIResult(c, fiber.StatusBadRequest, "User ID is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var messages []*types.Message
	if err := a.Database.DB.Preload("Sender.Avatar").Preload("Receiver.Avatar").Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)", claims.ULID, otherUserID, otherUserID, claims.ULID).Order("created_at ASC").Limit(100).Find(&messages).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type MessageResponse struct {
		ID            string `json:"id"`
		SenderID      string `json:"sender_id"`
		Sender        string `json:"sender"`
		SenderName    string `json:"sender_name"`
		SenderAvatar  string `json:"sender_avatar"`
		Content       string `json:"content"`
		CreatedAt     string `json:"created_at"`
		CurrentUserID string `json:"current_user_id"`
	}

	results := make([]*MessageResponse, 0, len(messages))
	for _, message := range messages {
		senderName := ""
		senderAvatar := "/assets/static/img/ui/placeholder_user.png"
		if message.Sender != nil {
			if message.Sender.Name != "" {
				senderName = message.Sender.Name
			} else {
				senderName = message.Sender.Username
			}
			if message.Sender.Avatar != nil && message.Sender.Avatar.Link != "" {
				senderAvatar = message.Sender.Avatar.Link
			}
		}
		results = append(results, &MessageResponse{
			ID:            message.ID,
			SenderID:      message.SenderID,
			Sender:        message.Sender.Username,
			SenderName:    senderName,
			SenderAvatar:  senderAvatar,
			Content:       message.Content,
			CreatedAt:     message.CreatedAt.Format("2006-01-02 15:04:05"),
			CurrentUserID: claims.ULID,
		})
	}

	return APIResult(c, fiber.StatusOK, "OK", results)
}

// SendMessage sends a chat message to another user
func (a *APIv1) SendMessage(c *fiber.Ctx) error {
	var args struct {
		ReceiverID string `json:"receiver_id" validate:"required,ulid"`
		Content    string `json:"content" validate:"required,max=1000"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if claims.ULID == args.ReceiverID {
		return APIResult(c, fiber.StatusBadRequest, "Cannot send message to yourself.", nil)
	}

	var friendship *types.Friend
	if err := a.Database.DB.First(&friendship, "user_id = ? AND friend_id = ?", claims.ULID, args.ReceiverID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return APIResult(c, fiber.StatusForbidden, "You can only send messages to friends.", nil)
		}
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	message := &types.Message{
		ID:         ulid.Make().String(),
		SenderID:   claims.ULID,
		ReceiverID: args.ReceiverID,
		Content:    args.Content,
	}
	if err := a.Database.DB.Create(message).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", message)
}

// RemoveFriend removes a friend
func (a *APIv1) RemoveFriend(c *fiber.Ctx) error {
	friendID := c.Params("id")
	if friendID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Friend ID is required.", nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if claims.ULID == friendID {
		return APIResult(c, fiber.StatusBadRequest, "Cannot remove yourself.", nil)
	}

	tx := a.Database.DB.Begin()
	if err := tx.Where("(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", claims.ULID, friendID, friendID, claims.ULID).Delete(&types.Friend{}).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// BlockUser blocks a user
func (a *APIv1) BlockUser(c *fiber.Ctx) error {
	var args struct {
		BlockedID string `json:"blocked_id" validate:"required,ulid"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if claims.ULID == args.BlockedID {
		return APIResult(c, fiber.StatusBadRequest, "Cannot block yourself.", nil)
	}

	var existingBlock *types.Blocklist
	err := a.Database.DB.First(&existingBlock, "user_id = ? AND blocked_id = ?", claims.ULID, args.BlockedID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}
	if existingBlock != nil && existingBlock.ID != "" {
		return APIResult(c, fiber.StatusBadRequest, "User is already blocked.", nil)
	}

	block := &types.Blocklist{
		ID:        ulid.Make().String(),
		UserID:    claims.ULID,
		BlockedID: args.BlockedID,
	}

	tx := a.Database.DB.Begin()

	if err := tx.Create(block).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Delete(&types.Friend{}, "(user_id = ? AND friend_id = ?) OR (user_id = ? AND friend_id = ?)", claims.ULID, args.BlockedID, args.BlockedID, claims.ULID).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Delete(&types.FriendRequest{}, "(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)", claims.ULID, args.BlockedID, args.BlockedID, claims.ULID).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// UnblockUser unblocks a user
func (a *APIv1) UnblockUser(c *fiber.Ctx) error {
	var args struct {
		BlockedID string `json:"blocked_id" validate:"required,ulid"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	if err := a.Database.DB.Delete(&types.Blocklist{}, "user_id = ? AND blocked_id = ?", claims.ULID, args.BlockedID).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", nil)
}

// GetBlocklist returns the blocklist for the current user
func (a *APIv1) GetBlocklist(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var blocks []*types.Blocklist
	if err := a.Database.DB.Preload("Blocked.Avatar").Where("user_id = ?", claims.ULID).Find(&blocks).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	type BlockedUserSummary struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Username  string `json:"username"`
		AvatarURL string `json:"avatar_url"`
	}

	results := make([]*BlockedUserSummary, 0, len(blocks))
	for _, block := range blocks {
		if block.Blocked != nil {
			avatarURL := "/assets/static/img/ui/placeholder_user.png"
			if block.Blocked.Avatar != nil && block.Blocked.Avatar.Link != "" {
				avatarURL = block.Blocked.Avatar.Link
			}
			results = append(results, &BlockedUserSummary{
				ID:        block.Blocked.ID,
				Name:      block.Blocked.Name,
				Username:  block.Blocked.Username,
				AvatarURL: avatarURL,
			})
		}
	}

	return APIResult(c, fiber.StatusOK, "OK", results)
}

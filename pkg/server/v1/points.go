package v1

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudlink-omega/backend/pkg/server/stripe"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

// PointsGetBalanceArgs represents the request for getting points balance.
type PointsGetBalanceArgs struct{}

// PointsCheckInArgs represents the request for daily check-in.
type PointsCheckInArgs struct{}

// PointsPurchaseCreateArgs represents the request for creating a points purchase.
type PointsPurchaseCreateArgs struct {
	Amount   int     `json:"amount" form:"amount" validate:"required,min=100,max=10000"`
	Price    float64 `json:"price" form:"price" validate:"required,min=1"`
	Currency string  `json:"currency" form:"currency" validate:"required"`
}

// PointsPurchaseConfirmArgs represents the request for confirming a points purchase.
type PointsPurchaseConfirmArgs struct {
	PurchaseID    string `json:"purchase_id" form:"purchase_id" validate:"required"`
	PaymentMethod string `json:"payment_method" form:"payment_method" validate:"required"`
	TransactionID string `json:"transaction_id" form:"transaction_id"`
	Password      string `json:"password" form:"password" validate:"required"`
}

// PointsDeductArgs represents the request for deducting points (from extensions).
type PointsDeductArgs struct {
	GameID       string `json:"game_id" form:"game_id" validate:"required"`
	Amount       int    `json:"amount" form:"amount" validate:"required,min=1"`
	Description  string `json:"description" form:"description"`
	RequestID    string `json:"request_id" form:"request_id" validate:"required"`
}

// PointsDeductConfirmArgs represents the request for confirming points deduction.
type PointsDeductConfirmArgs struct {
	RequestID string `json:"request_id" form:"request_id" validate:"required"`
	Password  string `json:"password" form:"password" validate:"required"`
}

// PointsTransferArgs represents the request for transferring points to a developer.
type PointsTransferArgs struct {
	ToUserID string `json:"to_user_id" form:"to_user_id" validate:"required"`
	Amount   int    `json:"amount" form:"amount" validate:"required,min=1"`
	GameID   string `json:"game_id" form:"game_id" validate:"required"`
	Reason   string `json:"reason" form:"reason"`
}

/*
 * Gets the user's points balance and check-in status.
 * GET /api/v1/points
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "balance": 1000,
 *     "last_check_in": "2024-01-01T00:00:00Z",
 *     "can_check_in": false
 *   }
 * }
 */
func (a *APIv1) GetPoints(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var userPoint *types.UserPoint
	if err := a.Database.DB.First(&userPoint, "user_id = ?", claims.ULID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  claims.ULID,
			Balance: 0,
		}
		a.Database.DB.Create(userPoint)
	}

	canCheckIn := false
	if userPoint.LastCheckIn.IsZero() || time.Since(userPoint.LastCheckIn) >= 24*time.Hour {
		canCheckIn = true
	}

	data := map[string]any{
		"balance":     userPoint.Balance,
		"last_check_in": userPoint.LastCheckIn.Format("2006-01-02T15:04:05Z"),
		"can_check_in": canCheckIn,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

func strPtr(s string) *string {
	return &s
}

/*
 * Performs daily check-in to earn points.
 * POST /api/v1/points/checkin
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "points_earned": 10,
 *     "new_balance": 1010,
 *     "next_check_in": "2024-01-02T00:00:00Z"
 *   }
 * }
 */
func (a *APIv1) CheckIn(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var userPoint *types.UserPoint
	if err := a.Database.DB.First(&userPoint, "user_id = ?", claims.ULID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  claims.ULID,
			Balance: 0,
		}
	}

	if !userPoint.LastCheckIn.IsZero() && time.Since(userPoint.LastCheckIn) < 24*time.Hour {
		return APIResult(c, fiber.StatusBadRequest, "Already checked in today.", nil)
	}

	pointsEarned := 10
	userPoint.Balance += pointsEarned
	userPoint.LastCheckIn = time.Now()
	userPoint.UpdatedAt = time.Now()

	if err := a.Database.DB.Save(userPoint).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	transaction := &types.PointTransaction{
		ID:        ulid.Make().String(),
		UserID:    claims.ULID,
		Amount:    pointsEarned,
		Type:      types.PointTransactionTypeEarn,
		Description: "Daily check-in reward",
		CreatedAt: time.Now(),
	}
	a.Database.DB.Create(transaction)

	data := map[string]any{
		"points_earned": pointsEarned,
		"new_balance":   userPoint.Balance,
		"next_check_in": userPoint.LastCheckIn.Add(24 * time.Hour).Format("2006-01-02T15:04:05Z"),
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

/*
 * Gets the user's points transaction history.
 * GET /api/v1/points/history
 *
 * Query Parameters:
 * - page: int (default: 1)
 * - limit: int (default: 50, max: 100)
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": [
 *     {
 *       "id": "transaction_id",
 *       "amount": 10,
 *       "type": "earn",
 *       "description": "Daily check-in reward",
 *       "created_at": "2024-01-01T00:00:00Z"
 *     }
 *   ]
 * }
 */
func (a *APIv1) GetPointsHistory(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
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

	var transactions []*types.PointTransaction
	if err := a.Database.DB.Where("user_id = ?", claims.ULID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", transactions)
}

/*
 * Creates a points purchase request.
 * POST /api/v1/points/purchase
 *
 * Body:
 * {
 *   "amount": 1000,
 *   "price": 9.99,
 *   "currency": "USD"
 * }
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "purchase_id": "purchase_id",
 *     "amount": 1000,
 *     "price": 9.99,
 *     "payment_url": "https://payment.example.com/pay/..."
 *   }
 * }
 */
func (a *APIv1) CreatePointsPurchase(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args PointsPurchaseCreateArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	if args.Amount < 100 || args.Amount > 10000 || args.Amount%100 != 0 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid amount. Must be between 100 and 10000, and divisible by 100.", nil)
	}

	cfg := stripe.LoadConfig()
	if !stripe.IsEnabled(cfg) {
		return APIResult(c, fiber.StatusServiceUnavailable, "Stripe payments are not configured.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, "Failed to retrieve user.", nil)
	}

	pointsAmount := args.Amount / 100
	successURL := fmt.Sprintf("%s/dashboard/points/success?session_id={CHECKOUT_SESSION_ID}", a.ParentServer.ServerURL)
	cancelURL := fmt.Sprintf("%s/dashboard/points/cancel", a.ParentServer.ServerURL)

	params := &stripe.CheckoutParams{
		Amount:         int64(args.Amount),
		Currency:       strings.ToLower(args.Currency),
		PointsAmount:   pointsAmount,
		SuccessURL:     successURL,
		CancelURL:      cancelURL,
		CustomerEmail:  user.Email,
		UserID:         claims.ULID,
	}

	checkoutURL, err := stripe.CreateCheckoutSession(params)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to create checkout session: %v", err), nil)
	}

	purchaseID := ulid.MustNew(ulid.Now(), nil).String()
	purchase := &types.PointPurchase{
		ID:        purchaseID,
		UserID:    claims.ULID,
		Amount:    pointsAmount,
		Price:     args.Price,
		Currency:  strings.ToUpper(args.Currency),
		Status:    types.PointPurchaseStatusPending,
		PaymentURL: *checkoutURL,
		PaymentMethod: strPtr("stripe"),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := a.Database.DB.Create(purchase).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	data := map[string]any{
		"purchase_id": purchase.ID,
		"amount":      purchase.Amount,
		"price":       purchase.Price,
		"currency":    purchase.Currency,
		"payment_url": *checkoutURL,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

/*
 * Confirms a points purchase after payment.
 * POST /api/v1/points/purchase/confirm
 *
 * Body:
 * {
 *   "purchase_id": "purchase_id",
 *   "payment_method": "credit_card",
 *   "transaction_id": "txn_123456",
 *   "password": "user_password"
 * }
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "new_balance": 1100
 *   }
 * }
 */
func (a *APIv1) ConfirmPointsPurchase(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args PointsPurchaseConfirmArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	var purchase *types.PointPurchase
	if err := a.Database.DB.First(&purchase, "id = ? AND user_id = ?", args.PurchaseID, claims.ULID).Error; err != nil {
		return APIResult(c, fiber.StatusNotFound, "Purchase not found.", nil)
	}

	if purchase.Status != types.PointPurchaseStatusPending {
		return APIResult(c, fiber.StatusBadRequest, "Purchase is not pending.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if !a.ParentServer.Accounts.DB.VerifyPassword(user, args.Password) {
		return APIResult(c, fiber.StatusUnauthorized, "Invalid password.", nil)
	}

	now := time.Now()
	purchase.Status = types.PointPurchaseStatusCompleted
	purchase.PaymentMethod = &args.PaymentMethod
	purchase.TransactionID = &args.TransactionID
	purchase.CompletedAt = &now
	purchase.UpdatedAt = now

	if err := a.Database.DB.Save(purchase).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var userPoint *types.UserPoint
	if err := a.Database.DB.First(&userPoint, "user_id = ?", claims.ULID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  claims.ULID,
			Balance: 0,
		}
	}

	userPoint.Balance += purchase.Amount
	userPoint.UpdatedAt = now
	if err := a.Database.DB.Save(userPoint).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	transaction := &types.PointTransaction{
		ID:        ulid.Make().String(),
		UserID:    claims.ULID,
		Amount:    purchase.Amount,
		Type:      types.PointTransactionTypePurchase,
		Description: fmt.Sprintf("Points purchase: %d points for %s %.2f", purchase.Amount, purchase.Currency, purchase.Price),
		CreatedAt: now,
	}
	a.Database.DB.Create(transaction)

	data := map[string]any{
		"new_balance": userPoint.Balance,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

/*
 * Requests a points deduction from a game/extension.
 * This creates a pending deduction request that requires user confirmation.
 * POST /api/v1/points/deduct
 *
 * Body:
 * {
 *   "game_id": "game_id",
 *   "amount": 100,
 *   "description": "Purchase item",
 *   "request_id": "unique_request_id"
 * }
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "confirmation_url": "https://omega.example.com/points/confirm/deduct/...",
 *     "request_id": "unique_request_id",
 *     "expires_at": "2024-01-01T00:15:00Z"
 *   }
 * }
 */
func (a *APIv1) RequestPointsDeduction(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args PointsDeductArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	if args.Amount < 1 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid amount.", nil)
	}

	var userPoint *types.UserPoint
	if err := a.Database.DB.First(&userPoint, "user_id = ?", claims.ULID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  claims.ULID,
			Balance: 0,
		}
	}

	if userPoint.Balance < args.Amount {
		return APIResult(c, fiber.StatusBadRequest, "Insufficient points balance.", nil)
	}

	if strings.TrimSpace(args.Description) == "" {
		args.Description = fmt.Sprintf("Points deduction for game %s", args.GameID)
	}

	confirmationToken := ulid.Make().String()
	expiresAt := time.Now().Add(15 * time.Minute)

	deductionRequest := &types.PointTransaction{
		ID:          confirmationToken,
		UserID:      claims.ULID,
		Amount:      -args.Amount,
		Type:        types.PointTransactionTypeSpend,
		Description: args.Description,
		RelatedGameID: &args.GameID,
		CreatedAt:   time.Now(),
	}

	if err := a.Database.DB.Create(deductionRequest).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	confirmationURL := fmt.Sprintf("%s/points/confirm/deduct/%s?request_id=%s&game_id=%s&amount=%d",
		a.ParentServer.ServerURL, confirmationToken, args.RequestID, args.GameID, args.Amount)

	data := map[string]any{
		"confirmation_url": confirmationURL,
		"request_id":       args.RequestID,
		"game_id":          args.GameID,
		"amount":           args.Amount,
		"description":      args.Description,
		"expires_at":       expiresAt.Format("2006-01-02T15:04:05Z"),
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

/*
 * Confirms a points deduction request.
 * POST /api/v1/points/deduct/confirm
 *
 * Body:
 * {
 *   "request_id": "unique_request_id",
 *   "password": "user_password"
 * }
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "new_balance": 900,
 *     "developer_id": "developer_id",
 *     "developer_points": 1100
 *   }
 * }
 */
func (a *APIv1) ConfirmPointsDeduction(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args PointsDeductConfirmArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	var deductionRequest *types.PointTransaction
	if err := a.Database.DB.First(&deductionRequest, "id = ? AND user_id = ? AND type = ?", args.RequestID, claims.ULID, types.PointTransactionTypeSpend).Error; err != nil {
		return APIResult(c, fiber.StatusNotFound, "Deduction request not found.", nil)
	}

	if deductionRequest.RelatedGameID == nil || *deductionRequest.RelatedGameID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Invalid deduction request.", nil)
	}

	user, err := a.ParentServer.Accounts.DB.GetUser(claims.ULID)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if !a.ParentServer.Accounts.DB.VerifyPassword(user, args.Password) {
		return APIResult(c, fiber.StatusUnauthorized, "Invalid password.", nil)
	}

	var game *types.DeveloperGame
	if err := a.Database.DB.First(&game, "id = ?", *deductionRequest.RelatedGameID).Error; err != nil {
		return APIResult(c, fiber.StatusNotFound, "Game not found.", nil)
	}

	if game.DeveloperID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Game has no developer.", nil)
	}

	var developer *types.Developer
	if err := a.Database.DB.First(&developer, "id = ?", game.DeveloperID).Error; err != nil {
		return APIResult(c, fiber.StatusNotFound, "Developer not found.", nil)
	}

	var developerMembers []*types.DeveloperMember
	if err := a.Database.DB.Where("developer_id = ? AND state IS NOT NULL", developer.ID).Find(&developerMembers).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var ownerUserID string
	for _, member := range developerMembers {
		if member.State.Read(1) {
			ownerUserID = member.UserID
			break
		}
	}

	if ownerUserID == "" {
		return APIResult(c, fiber.StatusBadRequest, "Developer has no owner.", nil)
	}

	tx := a.Database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var userPoint *types.UserPoint
	if err := tx.First(&userPoint, "user_id = ?", claims.ULID).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "User points not found.", nil)
	}

	if userPoint.Balance < -deductionRequest.Amount {
		tx.Rollback()
		return APIResult(c, fiber.StatusBadRequest, "Insufficient points balance.", nil)
	}

	userPoint.Balance += deductionRequest.Amount
	userPoint.UpdatedAt = time.Now()
	if err := tx.Save(userPoint).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	deductionRequest.CreatedAt = time.Now()
	if err := tx.Create(deductionRequest).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var developerPoint *types.UserPoint
	if err := tx.First(&developerPoint, "user_id = ?", ownerUserID).Error; err != nil {
		developerPoint = &types.UserPoint{
			UserID:  ownerUserID,
			Balance: 0,
		}
		tx.Create(developerPoint)
	}

	developerPoint.Balance += -deductionRequest.Amount
	developerPoint.UpdatedAt = time.Now()
	if err := tx.Save(developerPoint).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	transferTransaction := &types.PointTransaction{
		ID:             ulid.Make().String(),
		UserID:         claims.ULID,
		Amount:         deductionRequest.Amount,
		Type:           types.PointTransactionTypeTransfer,
		Description:    fmt.Sprintf("Transfer to developer %s for game %s", developer.Name, game.Name),
		RelatedUserID:  &ownerUserID,
		RelatedGameID:  &game.ID,
		CreatedAt:      time.Now(),
	}
	if err := tx.Create(transferTransaction).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	data := map[string]any{
		"new_balance":      userPoint.Balance,
		"developer_id":     developer.ID,
		"developer_name":   developer.Name,
		"developer_points": developerPoint.Balance,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

/*
 * Transfers points from one user to another (admin only or system use).
 * POST /api/v1/points/transfer
 *
 * Body:
 * {
 *   "to_user_id": "user_id",
 *   "amount": 100,
 *   "game_id": "game_id",
 *   "reason": "Game purchase"
 * }
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "from_balance": 900,
 *     "to_balance": 1100
 *   }
 * }
 */
func (a *APIv1) TransferPoints(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args PointsTransferArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	if args.Amount < 1 {
		return APIResult(c, fiber.StatusBadRequest, "Invalid amount.", nil)
	}

	if args.ToUserID == claims.ULID {
		return APIResult(c, fiber.StatusBadRequest, "Cannot transfer to self.", nil)
	}

	tx := a.Database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var fromUserPoint *types.UserPoint
	if err := tx.First(&fromUserPoint, "user_id = ?", claims.ULID).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, "User points not found.", nil)
	}

	if fromUserPoint.Balance < args.Amount {
		tx.Rollback()
		return APIResult(c, fiber.StatusBadRequest, "Insufficient points balance.", nil)
	}

	fromUserPoint.Balance -= args.Amount
	fromUserPoint.UpdatedAt = time.Now()
	if err := tx.Save(fromUserPoint).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var toUserPoint *types.UserPoint
	if err := tx.First(&toUserPoint, "user_id = ?", args.ToUserID).Error; err != nil {
		toUserPoint = &types.UserPoint{
			UserID:  args.ToUserID,
			Balance: 0,
		}
		if err := tx.Create(toUserPoint).Error; err != nil {
			tx.Rollback()
			return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
		}
	}

	toUserPoint.Balance += args.Amount
	toUserPoint.UpdatedAt = time.Now()
	if err := tx.Save(toUserPoint).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	spendTransaction := &types.PointTransaction{
		ID:             ulid.Make().String(),
		UserID:         claims.ULID,
		Amount:         -args.Amount,
		Type:           types.PointTransactionTypeTransfer,
		Description:    args.Reason,
		RelatedUserID:  &args.ToUserID,
		CreatedAt:      time.Now(),
	}
	if args.GameID != "" {
		gameID := args.GameID
		spendTransaction.RelatedGameID = &gameID
	}
	if err := tx.Create(spendTransaction).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	receiveTransaction := &types.PointTransaction{
		ID:             ulid.Make().String(),
		UserID:         args.ToUserID,
		Amount:         args.Amount,
		Type:           types.PointTransactionTypeTransfer,
		Description:    args.Reason,
		RelatedUserID:  &claims.ULID,
		CreatedAt:      time.Now(),
	}
	if args.GameID != "" {
		gameID := args.GameID
		receiveTransaction.RelatedGameID = &gameID
	}
	if err := tx.Create(receiveTransaction).Error; err != nil {
		tx.Rollback()
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	if err := tx.Commit().Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	data := map[string]any{
		"from_balance": fromUserPoint.Balance,
		"to_balance":   toUserPoint.Balance,
		"to_user_id":   args.ToUserID,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

/*
 * Processes a points purchase payment.
 * POST /api/v1/points/payment/process
 *
 * Body:
 * {
 *   "purchase_id": "purchase_id",
 *   "payment_method": "credit_card",
 *   "transaction_id": "txn_123456"
 * }
 *
 * Response: application/json
 * 200 OK
 * {
 *   "result": "OK",
 *   "data": {
 *     "new_balance": 1100
 *   }
 * }
 */
func (a *APIv1) ProcessPointsPayment(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args struct {
		PurchaseID    string  `json:"purchase_id" form:"purchase_id" validate:"required"`
		PaymentMethod string  `json:"payment_method" form:"payment_method" validate:"required"`
		TransactionID string  `json:"transaction_id" form:"transaction_id"`
	}
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, err.Error(), nil)
	}

	var purchase *types.PointPurchase
	if err := a.Database.DB.First(&purchase, "id = ? AND user_id = ?", args.PurchaseID, claims.ULID).Error; err != nil {
		return APIResult(c, fiber.StatusNotFound, "Purchase not found.", nil)
	}

	if purchase.Status != types.PointPurchaseStatusPending {
		return APIResult(c, fiber.StatusBadRequest, "Purchase is not pending.", nil)
	}

	now := time.Now()
	purchase.Status = types.PointPurchaseStatusCompleted
	purchase.PaymentMethod = &args.PaymentMethod
	if args.TransactionID != "" {
		purchase.TransactionID = &args.TransactionID
	}
	purchase.CompletedAt = &now
	purchase.UpdatedAt = now

	if err := a.Database.DB.Save(purchase).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	var userPoint *types.UserPoint
	if err := a.Database.DB.First(&userPoint, "user_id = ?", claims.ULID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  claims.ULID,
			Balance: 0,
		}
		a.Database.DB.Create(userPoint)
	}

	userPoint.Balance += purchase.Amount
	userPoint.UpdatedAt = now
	if err := a.Database.DB.Save(userPoint).Error; err != nil {
		return APIResult(c, fiber.StatusInternalServerError, err.Error(), nil)
	}

	transaction := &types.PointTransaction{
		ID:        ulid.Make().String(),
		UserID:    claims.ULID,
		Amount:    purchase.Amount,
		Type:      types.PointTransactionTypePurchase,
		Description: fmt.Sprintf("Points purchase: %d points for %s %.2f", purchase.Amount, purchase.Currency, purchase.Price),
		CreatedAt: now,
	}
	a.Database.DB.Create(transaction)

	data := map[string]any{
		"new_balance": userPoint.Balance,
		"purchase_id": purchase.ID,
		"amount":      purchase.Amount,
	}

	return APIResult(c, fiber.StatusOK, "OK", data)
}

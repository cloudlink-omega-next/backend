package server

import (
	"fmt"
	"time"

	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

// PointsConfirmDeduction renders the points deduction confirmation page.
func (s *Server) PointsConfirmDeduction(c *fiber.Ctx) error {
	token := c.Params("token")
	requestID := c.Query("request_id")
	gameID := c.Query("game_id")
	amount := c.QueryInt("amount", 0)

	if token == "" || requestID == "" || gameID == "" || amount <= 0 {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid deduction request.",
		})
	}

	var deductionRequest *types.PointTransaction
	if err := s.DB.DB.First(&deductionRequest, "id = ? AND type = ?", token, types.PointTransactionTypeSpend).Error; err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusNotFound,
			Message: "Deduction request not found.",
		})
	}

	if deductionRequest.UserID == "" {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid deduction request.",
		})
	}

	var game *types.DeveloperGame
	if err := s.DB.DB.First(&game, "id = ?", gameID).Error; err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusNotFound,
			Message: "Game not found.",
		})
	}

	var userPoint *types.UserPoint
	if err := s.DB.DB.First(&userPoint, "user_id = ?", deductionRequest.UserID).Error; err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusInternalServerError,
			Message: "User points not found.",
		})
	}

	data := map[string]any{
		"BaseURL":      s.ServerURL,
		"ServerName":   s.ServerName,
		"Title":        "Confirm Points Deduction",
		"Token":        token,
		"RequestID":    requestID,
		"GameID":       gameID,
		"Amount":       amount,
		"GameName":     game.Name,
		"CurrentBalance": userPoint.Balance,
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/layouts/modal", data)
}

// PointsConfirmPurchase renders the points purchase confirmation page.
func (s *Server) PointsConfirmPurchase(c *fiber.Ctx) error {
	purchaseID := c.Params("purchaseID")

	if purchaseID == "" {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid purchase request.",
		})
	}

	var purchase *types.PointPurchase
	if err := s.DB.DB.First(&purchase, "id = ?", purchaseID).Error; err != nil {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusNotFound,
			Message: "Purchase not found.",
		})
	}

	if purchase.UserID == "" {
		return s.ErrorPage(c, &fiber.Error{
			Code:    fiber.StatusBadRequest,
			Message: "Invalid purchase request.",
		})
	}

	data := map[string]any{
		"BaseURL":      s.ServerURL,
		"ServerName":   s.ServerName,
		"Title":        "Confirm Points Purchase",
		"PurchaseID":   purchaseID,
		"Amount":       purchase.Amount,
		"Price":        purchase.Price,
		"Currency":     purchase.Currency,
		"PaymentURL":   purchase.PaymentURL,
		"Status":       string(purchase.Status),
	}

	c.Context().SetContentType("text/html; charset=utf-8")
	return c.Render("views/layouts/modal", data)
}

// PointsProcessPayment processes a points purchase payment.
func (s *Server) PointsProcessPayment(c *fiber.Ctx) error {
	purchaseID := c.FormValue("purchase_id")
	paymentMethod := c.FormValue("payment_method")
	transactionID := c.FormValue("transaction_id")
	password := c.FormValue("password")

	if purchaseID == "" || paymentMethod == "" || password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"result": "error",
			"message": "Missing required fields.",
		})
	}

	var purchase *types.PointPurchase
	if err := s.DB.DB.First(&purchase, "id = ?", purchaseID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"result": "error",
			"message": "Purchase not found.",
		})
	}

	if purchase.Status != types.PointPurchaseStatusPending {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"result": "error",
			"message": "Purchase is not pending.",
		})
	}

	user, err := s.Accounts.DB.GetUser(purchase.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": "error",
			"message": "User not found.",
		})
	}

	if !s.Accounts.DB.VerifyPassword(user, password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"result": "error",
			"message": "Invalid password.",
		})
	}

	now := time.Now()
	purchase.Status = types.PointPurchaseStatusCompleted
	purchase.PaymentMethod = &paymentMethod
	if transactionID != "" {
		purchase.TransactionID = &transactionID
	}
	purchase.CompletedAt = &now
	purchase.UpdatedAt = now

	if err := s.DB.DB.Save(purchase).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": "error",
			"message": "Failed to update purchase.",
		})
	}

	var userPoint *types.UserPoint
	if err := s.DB.DB.First(&userPoint, "user_id = ?", purchase.UserID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  purchase.UserID,
			Balance: 0,
		}
		s.DB.DB.Create(userPoint)
	}

	userPoint.Balance += purchase.Amount
	userPoint.UpdatedAt = now
	if err := s.DB.DB.Save(userPoint).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"result": "error",
			"message": "Failed to update user points.",
		})
	}

	transaction := &types.PointTransaction{
		ID:        ulid.Make().String(),
		UserID:    purchase.UserID,
		Amount:    purchase.Amount,
		Type:      types.PointTransactionTypePurchase,
		Description: fmt.Sprintf("Points purchase: %d points for %s %.2f", purchase.Amount, purchase.Currency, purchase.Price),
		CreatedAt: now,
	}
	s.DB.DB.Create(transaction)

	return c.JSON(fiber.Map{
		"result": "OK",
		"data": fiber.Map{
			"new_balance": userPoint.Balance,
			"purchase_id": purchase.ID,
			"amount":      purchase.Amount,
		},
	})
}

// PaymentSuccess renders the payment success page
func (s *Server) PaymentSuccess(c *fiber.Ctx) error {
	sessionID := c.Query("session_id")
	data := fiber.Map{
		"session_id": sessionID,
		"IsAdmin":    s.IsAdmin(c),
	}
	return c.Render("views/payment_success", data, "views/layouts/default")
}

// PaymentCancelled renders the payment cancelled page
func (s *Server) PaymentCancelled(c *fiber.Ctx) error {
	data := fiber.Map{
		"IsAdmin": s.IsAdmin(c),
	}
	return c.Render("views/payment_cancelled", data, "views/layouts/default")
}

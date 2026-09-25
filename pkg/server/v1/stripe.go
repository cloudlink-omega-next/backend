package v1

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudlink-omega/backend/pkg/server/stripe"
	"github.com/cloudlink-omega/storage/pkg/types"
	"github.com/gofiber/fiber/v2"
	"github.com/oklog/ulid/v2"
)

// StripeWebhookArgs represents the parsed Stripe webhook event
type StripeWebhookArgs struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Data      WebhookData `json:"data"`
	Created   int64     `json:"created"`
}

type WebhookData struct {
	Object WebhookObject `json:"object"`
}

type WebhookObject struct {
	ID               string            `json:"id"`
	Object           string            `json:"object"`
	AmountTotal      int64             `json:"amount_total"`
	Currency         string            `json:"currency"`
	CustomerDetails  *CustomerDetails  `json:"customer_details"`
	PaymentStatus    string            `json:"payment_status"`
	Status           string            `json:"status"`
	Metadata         map[string]string `json:"metadata"`
}

type CustomerDetails struct {
	Email string `json:"email"`
}

// StripeCheckoutArgs represents the request for creating a Stripe checkout session
type StripeCheckoutArgs struct {
	Amount   int    `json:"amount" form:"amount" validate:"required,min=100,max=10000"`
	Currency string `json:"currency" form:"currency" validate:"required,currency"`
}

// CreateStripeCheckoutSession creates a Stripe Checkout Session for points purchase
func (a *APIv1) CreateStripeCheckoutSession(c *fiber.Ctx) error {
	claims := a.ParentServer.Authorization.GetNormalClaims(c)
	if claims == nil {
		return APIResult(c, fiber.StatusUnauthorized, "Unauthorized.", nil)
	}

	var args StripeCheckoutArgs
	if err := c.BodyParser(&args); err != nil {
		return APIResult(c, fiber.StatusBadRequest, "Invalid request body.", nil)
	}

	if args.Amount < 100 || args.Amount > 10000 || args.Amount%100 != 0 {
		return APIResult(c, fiber.StatusBadRequest, "Amount must be between 100 and 10000 and divisible by 100.", nil)
	}

	if args.Currency == "" {
		args.Currency = "usd"
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
		Metadata: map[string]string{
			"user_id":    claims.ULID,
			"points":     fmt.Sprintf("%d", pointsAmount),
			"username":   user.Username,
		},
	}

	checkoutURL, err := stripe.CreateCheckoutSession(params)
	if err != nil {
		return APIResult(c, fiber.StatusInternalServerError, fmt.Sprintf("Failed to create checkout session: %v", err), nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", map[string]any{
		"url": checkoutURL,
	})
}

// GetStripeConfig returns the Stripe publishable key for the frontend
func (a *APIv1) GetStripeConfig(c *fiber.Ctx) error {
	cfg := stripe.LoadConfig()
	if !stripe.IsEnabled(cfg) {
		return APIResult(c, fiber.StatusServiceUnavailable, "Stripe is not configured.", nil)
	}

	return APIResult(c, fiber.StatusOK, "OK", map[string]any{
		"publishable_key": cfg.PublishableKey,
	})
}

// StripeWebhook handles incoming Stripe webhook events
func (a *APIv1) StripeWebhook(c *fiber.Ctx) error {
	cfg := stripe.LoadConfig()
	if !stripe.IsEnabled(cfg) {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Stripe webhook is not configured",
		})
	}

	payload := c.Body()
	signatureHeader := c.Get("Stripe-Signature")

	if err := stripe.VerifyWebhookSignature(payload, signatureHeader, cfg.WebhookSecret); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Webhook signature verification failed: %v", err),
		})
	}

	var event StripeWebhookArgs
	if err := json.Unmarshal(payload, &event); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to parse webhook event: %v", err),
		})
	}

	switch event.Type {
	case "checkout.session.completed":
		return a.handleCheckoutSessionCompleted(event.Data.Object)
	case "checkout.session.async_payment_succeeded":
		return a.handleCheckoutSessionAsyncPaymentSucceeded(event.Data.Object)
	case "checkout.session.async_payment_failed":
		return a.handleCheckoutSessionAsyncPaymentFailed(event.Data.Object)
	default:
		return c.JSON(fiber.Map{
			"received": true,
		})
	}
}

func (a *APIv1) handleCheckoutSessionCompleted(object WebhookObject) error {
	userID := object.Metadata["user_id"]
	pointsStr := object.Metadata["points"]
	username := object.Metadata["username"]

	if userID == "" || pointsStr == "" {
		return fmt.Errorf("missing metadata in checkout session")
	}

	var pointsAmount int
	if _, err := fmt.Sscanf(pointsStr, "%d", &pointsAmount); err != nil {
		return fmt.Errorf("invalid points amount in metadata: %v", err)
	}

	if pointsAmount <= 0 {
		return fmt.Errorf("invalid points amount: %d", pointsAmount)
	}

	var userPoint *types.UserPoint
	if err := a.Database.DB.First(&userPoint, "user_id = ?", userID).Error; err != nil {
		userPoint = &types.UserPoint{
			UserID:  userID,
			Balance: 0,
		}
		a.Database.DB.Create(userPoint)
	}

	userPoint.Balance += pointsAmount
	if err := a.Database.DB.Save(userPoint).Error; err != nil {
		return fmt.Errorf("failed to update user points balance: %v", err)
	}

	purchaseID := ulid.MustNew(ulid.Now(), nil).String()
	purchase := &types.PointPurchase{
		ID:        purchaseID,
		UserID:    userID,
		Amount:    pointsAmount,
		Price:     float64(object.AmountTotal) / 100.0,
		Currency:  strings.ToLower(object.Currency),
		Status:    types.PointPurchaseStatusCompleted,
		PaymentMethod: strPtr("stripe"),
		TransactionID: &object.ID,
		CompletedAt: timePtr(time.Now()),
	}

	if err := a.Database.DB.Create(purchase).Error; err != nil {
		return fmt.Errorf("failed to create point purchase record: %v", err)
	}

	transactionID := ulid.MustNew(ulid.Now(), nil).String()
	description := fmt.Sprintf("Purchased %d points via Stripe (transaction: %s)", pointsAmount, object.ID)
	if username != "" {
		description = fmt.Sprintf("Purchased %d points via Stripe by %s (transaction: %s)", pointsAmount, username, object.ID)
	}

	transaction := &types.PointTransaction{
		ID:        transactionID,
		UserID:    userID,
		Amount:    pointsAmount,
		Type:      types.PointTransactionTypePurchase,
		Description: description,
		CreatedAt: time.Now(),
	}

	if err := a.Database.DB.Create(transaction).Error; err != nil {
		return fmt.Errorf("failed to create point transaction record: %v", err)
	}

	return nil
}

func (a *APIv1) handleCheckoutSessionAsyncPaymentSucceeded(object WebhookObject) error {
	if object.PaymentStatus == "paid" && object.Status == "complete" {
		return a.handleCheckoutSessionCompleted(object)
	}
	return nil
}

func (a *APIv1) handleCheckoutSessionAsyncPaymentFailed(object WebhookObject) error {
	return nil
}

func timePtr(t time.Time) *time.Time {
	return &t
}

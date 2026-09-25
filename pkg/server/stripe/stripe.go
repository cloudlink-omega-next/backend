package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/checkout/session"
)

type Config struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
	Enabled        bool
}

func LoadConfig() *Config {
	cfg := &Config{
		SecretKey:      os.Getenv("STRIPE_SECRET_KEY"),
		PublishableKey: os.Getenv("STRIPE_PUBLISHABLE_KEY"),
		WebhookSecret:  os.Getenv("STRIPE_WEBHOOK_SECRET"),
		Enabled:        os.Getenv("STRIPE_ENABLED") == "true",
	}
	return cfg
}

func Init(cfg *Config) {
	if cfg.Enabled && cfg.SecretKey != "" {
		stripe.Key = cfg.SecretKey
	}
}

func IsEnabled(cfg *Config) bool {
	return cfg != nil && cfg.Enabled && cfg.SecretKey != "" && cfg.PublishableKey != "" && cfg.WebhookSecret != ""
}

func VerifyWebhookSignature(payload []byte, signatureHeader string, webhookSecret string) error {
	if webhookSecret == "" {
		return fmt.Errorf("stripe webhook secret is not configured")
	}

	if signatureHeader == "" {
		return fmt.Errorf("stripe signature header is missing")
	}

	parts := strings.Split(signatureHeader, ",")
	if len(parts) == 0 {
		return fmt.Errorf("stripe signature header is invalid")
	}

	var timestamp string
	var signatures []string
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("stripe signature header part is invalid")
		}
		if kv[0] == "t" {
			timestamp = kv[1]
		} else if kv[0] == "v1" {
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return fmt.Errorf("stripe signature header is missing timestamp or signature")
	}

	signedPayload := timestamp + "." + string(payload)

	for _, signature := range signatures {
		mac := hmac.New(sha256.New, []byte(webhookSecret))
		mac.Write([]byte(signedPayload))
		expected := hex.EncodeToString(mac.Sum(nil))

		if hmac.Equal([]byte(signature), []byte(expected)) {
			return nil
		}
	}

	return fmt.Errorf("stripe signature verification failed")
}

type CheckoutParams struct {
	Amount       int64
	Currency     string
	PointsAmount int
	SuccessURL   string
	CancelURL    string
	CustomerEmail string
	UserID       string
	Metadata     map[string]string
}

func CreateCheckoutSession(params *CheckoutParams) (*string, error) {
	paramsMap := &stripe.CheckoutSessionParams{
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String(params.Currency),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String("Points Purchase"),
					},
					UnitAmount: stripe.Int64(params.Amount),
				},
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(params.SuccessURL),
		CancelURL:  stripe.String(params.CancelURL),
	}

	if params.CustomerEmail != "" {
		paramsMap.CustomerEmail = stripe.String(params.CustomerEmail)
	}

	if len(params.Metadata) > 0 {
		paramsMap.Metadata = params.Metadata
	}

	s, err := session.New(paramsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create stripe checkout session: %w", err)
	}

	return &s.URL, nil
}

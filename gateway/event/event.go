// Package event define los tipos Go del evento v1 (schema/event.v1.schema.json).
package event

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

const SchemaVersion = 1

// Tipos de evento.
const (
	TypeLoginAttempt       = "login_attempt"
	TypeAccountCreated     = "account_created"
	TypeCartAction         = "cart_action"
	TypeTransaction        = "transaction"
	TypeRead               = "read"
	TypeTransactionOutcome = "transaction_outcome"
)

// Decisiones del Gateway.
const (
	DecisionAllow  = "allow"
	DecisionReview = "review"
	DecisionStepUp = "step_up"
	DecisionBlock  = "block"
)

// Event es el evento completo: campos del cliente más campos del Gateway.
type Event struct {
	SchemaVersion   int       `json:"schema_version"`
	EventType       string    `json:"event_type"`
	ClientRequestID string    `json:"client_request_id"`
	OccurredAt      time.Time `json:"occurred_at"`
	SessionID       string    `json:"session_id,omitempty"`
	Identity        Identity  `json:"identity"`
	Payload         any       `json:"payload"`

	EventID       string         `json:"event_id"`
	TraceID       string         `json:"trace_id"`
	ReceivedAt    time.Time      `json:"received_at"`
	ActorID       string         `json:"actor_id"`
	ActorIDSource string         `json:"actor_id_source"`
	Decision      string         `json:"decision"`
	RulesFired    []string       `json:"rules_fired"` // nunca nil: el esquema exige un array
	Enrichment    map[string]any `json:"enrichment,omitempty"`
}

type Identity struct {
	UserID    string `json:"user_id,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	IPAddress string `json:"ip_address"`
}

// Money es un importe en unidades menores (centavos).
type Money struct {
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

type LineItem struct {
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	UnitPrice Money  `json:"unit_price"`
}

type LoginAttemptPayload struct {
	Success       bool   `json:"success"`
	Method        string `json:"method"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type AccountCreatedPayload struct {
	RegistrationChannel string `json:"registration_channel"`
	EmailDomain         string `json:"email_domain"`
	ReferralCode        string `json:"referral_code,omitempty"`
}

type CartActionPayload struct {
	Action    string `json:"action"`
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	UnitPrice *Money `json:"unit_price,omitempty"`
}

type TransactionPayload struct {
	Amount              Money      `json:"amount"`
	Items               []LineItem `json:"items"`
	PaymentMethod       string     `json:"payment_method"`
	PaymentInstrumentID string     `json:"payment_instrument_id"`
	ShippingCountry     string     `json:"shipping_country,omitempty"`
	PromoCode           string     `json:"promo_code,omitempty"`
}

type ReadPayload struct {
	ResourceType    string `json:"resource_type"`
	ResourceID      string `json:"resource_id"`
	ResourceOwnerID string `json:"resource_owner_id,omitempty"`
}

type TransactionOutcomePayload struct {
	OriginalClientRequestID string `json:"original_client_request_id,omitempty"`
	OriginalEventID         string `json:"original_event_id,omitempty"`
	Outcome                 string `json:"outcome"`
	FailureReason           string `json:"failure_reason,omitempty"`
}

// UnmarshalJSON decodifica el payload según event_type y rechaza campos desconocidos.
func (e *Event) UnmarshalJSON(data []byte) error {
	type plain Event // evita la recursión
	aux := struct {
		*plain
		Payload json.RawMessage `json:"payload"`
	}{plain: (*plain)(e)}
	if err := strictUnmarshal(data, &aux); err != nil {
		return err
	}

	var p any
	switch e.EventType {
	case TypeLoginAttempt:
		p = new(LoginAttemptPayload)
	case TypeAccountCreated:
		p = new(AccountCreatedPayload)
	case TypeCartAction:
		p = new(CartActionPayload)
	case TypeTransaction:
		p = new(TransactionPayload)
	case TypeRead:
		p = new(ReadPayload)
	case TypeTransactionOutcome:
		p = new(TransactionOutcomePayload)
	default:
		return fmt.Errorf("event_type desconocido: %q", e.EventType)
	}
	if err := strictUnmarshal(aux.Payload, p); err != nil {
		return fmt.Errorf("payload de %s: %w", e.EventType, err)
	}
	e.Payload = p
	return nil
}

func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

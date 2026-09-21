package mock

import (
	"time"

	"github.com/foomo/sesamy-go/pkg/encoding/mpv2"
	"github.com/foomo/sesamy-go/pkg/sesamy"
)

// Fixed identifiers shared by every mock payload.
const (
	DefaultClientID           = "584335997.1746564151"
	DefaultUserID             = "U-1000"
	DefaultSessionID          = "1749196701"
	DefaultEngagementTimeMSec = 10086
)

// BaseTime is the point in time every mock payload is stamped with. Mocks are
// deterministic by design, so that the generated testdata fixtures do not churn between
// runs. Note that mpv2.NewPayload stamps time.Now() and is therefore deliberately unused
// here.
var BaseTime = time.Date(2026, time.September, 10, 9, 18, 19, 0, time.UTC)

// Context holds the envelope fields of an mpv2.Payload. They do not depend on the event
// param type, so options can be written once here rather than per generic instantiation.
type Context struct {
	ClientID           string
	UserID             string
	SessionID          string
	TimestampMicros    int64
	UserProperties     map[string]any
	Consent            *mpv2.ConsentData
	UserData           *mpv2.UserData
	DebugMode          bool
	EngagementTimeMSec int64
}

// Option overrides a single envelope field.
type Option func(*Context)

// NewContext resolves the defaults and applies opts on top.
func NewContext(opts ...Option) Context {
	c := Context{
		ClientID:           DefaultClientID,
		UserID:             DefaultUserID,
		SessionID:          DefaultSessionID,
		TimestampMicros:    BaseTime.UnixMicro(),
		Consent:            ConsentGranted(),
		EngagementTimeMSec: DefaultEngagementTimeMSec,
	}

	for _, opt := range opts {
		opt(&c)
	}

	return c
}

func WithClientID(v string) Option {
	return func(c *Context) { c.ClientID = v }
}

func WithUserID(v string) Option {
	return func(c *Context) { c.UserID = v }
}

// WithoutUserID models an anonymous visitor, who has a client id but no account.
func WithoutUserID() Option {
	return func(c *Context) { c.UserID = "" }
}

func WithSessionID(v string) Option {
	return func(c *Context) { c.SessionID = v }
}

func WithTimestamp(v time.Time) Option {
	return func(c *Context) { c.TimestampMicros = v.UnixMicro() }
}

func WithUserProperties(v map[string]any) Option {
	return func(c *Context) { c.UserProperties = v }
}

func WithConsent(v *mpv2.ConsentData) Option {
	return func(c *Context) { c.Consent = v }
}

func WithConsentGranted() Option {
	return WithConsent(ConsentGranted())
}

func WithConsentDenied() Option {
	return WithConsent(ConsentDenied())
}

// WithoutConsent models traffic that arrived before a consent decision was made.
func WithoutConsent() Option {
	return WithConsent(nil)
}

func WithUserData(v *mpv2.UserData) Option {
	return func(c *Context) { c.UserData = v }
}

func WithDebugMode() Option {
	return func(c *Context) { c.DebugMode = true }
}

func WithEngagementTime(v time.Duration) Option {
	return func(c *Context) { c.EngagementTimeMSec = v.Milliseconds() }
}

// ConsentGranted returns a consent block with every category granted.
func ConsentGranted() *mpv2.ConsentData {
	return consent(mpv2.ConsentGranted)
}

// ConsentDenied returns a consent block with every category denied.
func ConsentDenied() *mpv2.ConsentData {
	return consent(mpv2.ConsentDenied)
}

// consent fills every category with v. Each field gets its own pointer, so a caller
// tweaking one category does not silently change the rest.
func consent(v mpv2.Consent) *mpv2.ConsentData {
	return &mpv2.ConsentData{
		AdStorage:              new(v),
		AdUserData:             new(v),
		AdPersonalization:      new(v),
		AnalyticsStorage:       new(v),
		FunctionalityStorage:   new(v),
		PersonalizationStorage: new(v),
		SecurityStorage:        new(v),
	}
}

// Payload wraps events in an mpv2 envelope carrying the resolved context. It is generic
// over the param type, so it serves both the typed single-event payloads and the mixed
// sesamy.Event[any] journeys.
func Payload[P any](events []sesamy.Event[P], opts ...Option) mpv2.Payload[P] {
	c := NewContext(opts...)

	return mpv2.Payload[P]{
		ClientID:           c.ClientID,
		UserID:             c.UserID,
		TimestampMicros:    c.TimestampMicros,
		UserProperties:     c.UserProperties,
		Consent:            c.Consent,
		Events:             events,
		UserData:           c.UserData,
		DebugMode:          c.DebugMode,
		SessionID:          c.SessionID,
		EngagementTimeMSec: c.EngagementTimeMSec,
	}
}

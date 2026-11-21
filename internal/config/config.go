package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	TelegramToken         string
	LogLevel              string
	WordsFile             string
	GrammarQuestionsFile  string
	GrammarCategoriesFile string
	SchedulerTime         string // daily summary send time, format "HH:MM"
	Timezone              string // IANA timezone name
	MaxRoundsPerDay       int    // max daily-task rounds a user can run per day
	AdminToken            string // bearer token for /admin/* endpoints; empty disables them

	// Telegram delivery. Exactly one of WebhookURL and TelegramLocalMode is
	// set; ValidateWebhook refuses both and neither.
	WebhookURL string
	// TelegramWebhookSecret is sent to Telegram as setWebhook's secret_token
	// and required back in X-Telegram-Bot-Api-Secret-Token on every POST
	// /webhook. Mandatory whenever WebhookURL is set: without it anyone who
	// knows the URL can post an update on behalf of any user.
	TelegramWebhookSecret string
	// TelegramLocalMode starts the bot without touching its webhook. It exists
	// because an empty WebhookURL used to reach setWebhook and delete the
	// webhook of whichever bot's token was configured, production included.
	TelegramLocalMode bool
	// TelegramInitTimeout bounds the getMe call go-telegram/bot makes inside
	// bot.New. The library's own 5s failed deploys while Telegram was merely
	// slow for one bot.
	TelegramInitTimeout time.Duration

	// Web UI. Empty TelegramClientID disables the web interface entirely: the bot
	// keeps working, /login just isn't mounted. That mirrors pochitushki, where
	// web login and the bot are independent of each other.
	AppBaseURL           string // public origin of the web UI, e.g. https://makeme.example.com
	TelegramClientID     string // OAuth client id from Telegram
	TelegramClientSecret string // OAuth client secret from Telegram
	TelegramOAuthBaseURL string // OIDC issuer base, overridable so tests can point at a stub
	// TelegramExpectedIssuer is the exact iss the id_token must carry. It
	// defaults to Telegram's own issuer and is set separately from the base URL
	// on purpose: pointing the flow at a local login broker must be a deliberate
	// change to both, not something one edit quietly enables.
	TelegramExpectedIssuer string
	JWTSecret              string // base64, must decode to >=32 bytes for HS256
	JWTTTLHours            int    // lifetime of the auth_token cookie and the JWT inside it
	CookieSecure           bool   // false only for local http development
}

// WebEnabled reports whether the web UI has everything it needs to be mounted.
func (c *Config) WebEnabled() bool {
	return c.TelegramClientID != "" && c.TelegramClientSecret != "" &&
		c.JWTSecret != "" && c.AppBaseURL != ""
}

// RedirectURI is the OAuth callback Telegram sends the user back to.
func (c *Config) RedirectURI() string {
	return strings.TrimRight(c.AppBaseURL, "/") + "/auth/telegram/callback"
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		TelegramToken:         os.Getenv("TELEGRAM_TOKEN"),
		WebhookURL:            os.Getenv("WEBHOOK_URL"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		WordsFile:             getEnv("WORDS_FILE", "data/words.json"),
		GrammarQuestionsFile:  getEnv("GRAMMAR_QUESTIONS_FILE", "data/grammar.json"),
		GrammarCategoriesFile: getEnv("GRAMMAR_CATEGORIES_FILE", "data/grammar_categories.json"),
		SchedulerTime:         getEnv("SCHEDULER_TIME", "21:00"),
		Timezone:              getEnv("TIMEZONE", "Europe/Moscow"),
		MaxRoundsPerDay:       getEnvInt("MAX_ROUNDS_PER_DAY", 5),
		AdminToken:            os.Getenv("ADMIN_TOKEN"),

		AppBaseURL:             os.Getenv("APP_BASE_URL"),
		TelegramClientID:       os.Getenv("TELEGRAM_CLIENT_ID"),
		TelegramClientSecret:   os.Getenv("TELEGRAM_CLIENT_SECRET"),
		TelegramOAuthBaseURL:   getEnv("TELEGRAM_OAUTH_BASE_URL", "https://oauth.telegram.org"),
		TelegramExpectedIssuer: getEnv("TELEGRAM_EXPECTED_ISSUER", "https://oauth.telegram.org"),
		JWTSecret:              os.Getenv("JWT_SECRET"),
		JWTTTLHours:            getEnvInt("JWT_TTL_HOURS", 24),
		CookieSecure:           getEnvBool("COOKIE_SECURE", true),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("TELEGRAM_TOKEN is required")
	}
	localMode, err := getEnvBoolStrict("TELEGRAM_LOCAL_MODE")
	if err != nil {
		return nil, err
	}
	cfg.TelegramLocalMode = localMode
	initTimeout, err := getEnvDuration("TELEGRAM_INIT_TIMEOUT", DefaultTelegramInitTimeout)
	if err != nil {
		return nil, err
	}
	cfg.TelegramInitTimeout = initTimeout

	return cfg, nil
}

// webhookSecretPattern is Telegram's own constraint on setWebhook's
// secret_token. A value outside it would be rejected by setWebhook only after
// migrations and seeding, so it is checked before anything else runs.
var webhookSecretPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// ValidateWebhook checks how the server receives updates. It is separate from
// Load because only cmd/makeme serves the webhook: cmd/seed and cmd/seed-audio
// load the same environment for DATABASE_URL and must not fail over settings
// they never use. The server calls it right after Load, before migrations and
// before any Telegram call.
func (c *Config) ValidateWebhook() error {
	if c.TelegramWebhookSecret != "" && !webhookSecretPattern.MatchString(c.TelegramWebhookSecret) {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET must be 1-256 characters of A-Z, a-z, 0-9, _ or - (generate one with: openssl rand -hex 32)")
	}
	if c.TelegramLocalMode && c.WebhookURL != "" {
		return fmt.Errorf("TELEGRAM_LOCAL_MODE=true and WEBHOOK_URL are mutually exclusive: local mode leaves the bot's webhook untouched, WEBHOOK_URL replaces it; unset one of them")
	}
	if !c.TelegramLocalMode && c.WebhookURL == "" {
		return fmt.Errorf("WEBHOOK_URL is required: with it empty, setWebhook would delete the webhook of the bot whose token is in TELEGRAM_TOKEN. For local development without a webhook set TELEGRAM_LOCAL_MODE=true")
	}
	if c.WebhookURL != "" {
		u, err := url.Parse(c.WebhookURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" {
			return fmt.Errorf("WEBHOOK_URL must be an absolute https URL with a host, got %q: setWebhook with a malformed URL can drop the current webhook before Telegram rejects it", c.WebhookURL)
		}
	}
	if c.WebhookURL != "" && c.TelegramWebhookSecret == "" {
		return fmt.Errorf("TELEGRAM_WEBHOOK_SECRET is required when WEBHOOK_URL is set: without it Telegram's updates cannot be told from forged ones (generate one with: openssl rand -hex 32)")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

// DefaultTelegramInitTimeout is how long startup waits for getMe. Six times
// the library's 5s, still well inside krill's 60s healthcheck start period
// together with migrations and seeding (~7s on production).
const DefaultTelegramInitTimeout = 30 * time.Second

// getEnvDuration reads a positive Go duration ("30s", "1m"). Unlike getEnvInt
// it rejects a bad value instead of falling back: a timeout that silently
// reverts to its default is the problem this setting exists to solve.
func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 30s or 1m, got %q", key, v)
	}
	return d, nil
}

// getEnvBoolStrict reads an opt-in flag: unset means false, anything that
// strconv.ParseBool rejects is an error rather than a silent false, so a typo
// is reported instead of being taken for "no".
func getEnvBoolStrict(key string) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, v)
	}
	return b, nil
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

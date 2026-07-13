package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type config struct {
	Port                                     string `json:"port"`
	MongoURI                                 string `json:"-"`
	MongoDatabase                            string `json:"mongoDatabase"`
	SessionSecret                            string `json:"-"`
	CookieSecure                             bool   `json:"-"`
	BackendPublicURL                         string `json:"backendPublicUrl"`
	KeycloakBaseURL                          string `json:"keycloakBaseUrl"`
	KeycloakPublicURL                        string `json:"keycloakPublicUrl"`
	KeycloakRealm                            string `json:"keycloakRealm"`
	KeycloakClientID                         string `json:"keycloakClientId"`
	KeycloakClientSecret                     string `json:"-"`
	OperationKeycloakBaseURL                 string `json:"operationKeycloakBaseUrl"`
	OperationKeycloakRealm                   string `json:"operationKeycloakRealm"`
	OperationKeycloakClientID                string `json:"operationKeycloakClientId"`
	OperationKeycloakClientSecret            string `json:"-"`
	OperationTokenExchangeEnabled            bool   `json:"operationTokenExchangeEnabled"`
	OperationTokenExchangeAudience           string `json:"operationTokenExchangeAudience"`
	OperationTokenExchangeRequestedIssuer    string `json:"operationTokenExchangeRequestedIssuer"`
	OperationTokenExchangeRequestedTokenType string `json:"operationTokenExchangeRequestedTokenType"`
	SupersetPublicURL                        string `json:"supersetPublicUrl"`
	SupersetOperationURL                     string `json:"supersetOperationUrl"`
}

type session struct {
	ID                    string    `bson:"_id" json:"id"`
	UserID                string    `bson:"userId" json:"userId"`
	Username              string    `bson:"username" json:"username"`
	Email                 string    `bson:"email" json:"email"`
	Zone                  string    `bson:"zone" json:"zone"`
	EncryptedAccessToken  string    `bson:"encryptedAccessToken" json:"-"`
	EncryptedRefreshToken string    `bson:"encryptedRefreshToken" json:"-"`
	TokenExpiresAt        time.Time `bson:"tokenExpiresAt" json:"tokenExpiresAt"`
	CreatedAt             time.Time `bson:"createdAt" json:"createdAt"`
	ExpiresAt             time.Time `bson:"expiresAt" json:"expiresAt"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type userInfo struct {
	Subject           string `json:"sub"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
}

const (
	sessionCookieName  = "BI_ENGINE_SESSION"
	stateCookieName    = "BI_ENGINE_OAUTH_STATE"
	returnToCookieName = "BI_ENGINE_RETURN_TO"
)

// main نقطه شروع Go proxy است.
// این تابع تنظیمات را می‌خواند، به MongoDB وصل می‌شود، رمزنگاری توکن‌ها را آماده می‌کند
// و تمام routeهای اصلی احراز هویت، session، health check و proxy بین Public و Operation را ثبت می‌کند.
func main() {
	cfg := loadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("connect mongo: %v", err)
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()

	sessions := mongoClient.Database(cfg.MongoDatabase).Collection("sessions")
	codec, err := newTokenCodec(cfg.SessionSecret)
	if err != nil {
		log.Fatalf("configure token encryption: %v", err)
	}

	app := fiber.New(fiber.Config{
		AppName: "bi-engine-super-app-backend",
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer pingCancel()

		if err := mongoClient.Ping(pingCtx, nil); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "degraded",
				"mongo":  err.Error(),
			})
		}

		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Get("/config", func(c *fiber.Ctx) error {
		return c.JSON(cfg)
	})

	app.Get("/superset/zones", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"zones": []fiber.Map{
				{"name": "public", "url": "/superset/public/"},
				{"name": "operation", "url": "/api/superset/operation/api/"},
			},
		})
	})

	app.Get("/auth/login", func(c *fiber.Ctx) error {
		state, err := randomString(32)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "state generation failed"})
		}

		c.Cookie(&fiber.Cookie{
			Name:     stateCookieName,
			Value:    state,
			HTTPOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
			MaxAge:   300,
		})
		if returnTo := safeReturnTo(c.Query("return_to")); returnTo != "" {
			c.Cookie(&fiber.Cookie{
				Name:     returnToCookieName,
				Value:    returnTo,
				HTTPOnly: true,
				Secure:   cfg.CookieSecure,
				SameSite: fiber.CookieSameSiteLaxMode,
				Path:     "/",
				MaxAge:   300,
			})
		}

		values := url.Values{}
		values.Set("client_id", cfg.KeycloakClientID)
		values.Set("response_type", "code")
		values.Set("scope", "openid profile email")
		values.Set("redirect_uri", cfg.redirectURI())
		values.Set("state", state)

		return c.Redirect(cfg.keycloakPublicRealmURL()+"/protocol/openid-connect/auth?"+values.Encode(), fiber.StatusFound)
	})

	app.Get("/auth/callback", func(c *fiber.Ctx) error {
		if c.Query("state") == "" || c.Query("state") != c.Cookies(stateCookieName) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid OAuth state"})
		}
		code := c.Query("code")
		if code == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing authorization code"})
		}

		tokens, err := exchangeCode(cfg, code)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}
		info, err := fetchUserInfo(cfg, tokens.AccessToken)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
		}

		encryptedAccessToken, err := codec.encrypt(tokens.AccessToken)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token encryption failed"})
		}
		encryptedRefreshToken, err := codec.encrypt(tokens.RefreshToken)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "token encryption failed"})
		}
		sessionID, err := randomString(32)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "session generation failed"})
		}

		now := time.Now().UTC()
		username := info.PreferredUsername
		if username == "" {
			username = info.Subject
		}
		doc := session{
			ID:                    sessionIDHash(sessionID),
			UserID:                info.Subject,
			Username:              username,
			Email:                 info.Email,
			Zone:                  "operation",
			EncryptedAccessToken:  encryptedAccessToken,
			EncryptedRefreshToken: encryptedRefreshToken,
			TokenExpiresAt:        now.Add(time.Duration(tokens.ExpiresIn) * time.Second),
			CreatedAt:             now,
			ExpiresAt:             now.Add(8 * time.Hour),
		}

		insertCtx, insertCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer insertCancel()
		if _, err := sessions.InsertOne(insertCtx, doc); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		c.Cookie(&fiber.Cookie{
			Name:     sessionCookieName,
			Value:    sessionID,
			HTTPOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
			MaxAge:   8 * 60 * 60,
		})
		c.Cookie(&fiber.Cookie{
			Name:     stateCookieName,
			Value:    "",
			HTTPOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
			MaxAge:   -1,
		})
		returnTo := safeReturnTo(c.Cookies(returnToCookieName))
		c.Cookie(&fiber.Cookie{
			Name:     returnToCookieName,
			Value:    "",
			HTTPOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
			MaxAge:   -1,
		})
		if returnTo == "" {
			returnTo = "/superset-mfe/"
		}

		return c.Redirect(returnTo, fiber.StatusFound)
	})

	app.Get("/auth/me", func(c *fiber.Ctx) error {
		doc, err := loadSession(c, sessions)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"authenticated": false})
		}
		return c.JSON(fiber.Map{
			"authenticated": true,
			"userId":        doc.UserID,
			"username":      doc.Username,
			"email":         doc.Email,
			"expiresAt":     doc.ExpiresAt,
		})
	})

	app.Post("/auth/logout", func(c *fiber.Ctx) error {
		if sessionID := c.Cookies(sessionCookieName); sessionID != "" {
			deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer deleteCancel()
			_, _ = sessions.DeleteOne(deleteCtx, bson.M{"_id": sessionIDHash(sessionID)})
		}
		c.Cookie(&fiber.Cookie{
			Name:     sessionCookieName,
			Value:    "",
			HTTPOnly: true,
			Secure:   cfg.CookieSecure,
			SameSite: fiber.CookieSameSiteLaxMode,
			Path:     "/",
			MaxAge:   -1,
		})
		return c.JSON(fiber.Map{"ok": true})
	})

	app.All("/superset/operation/*", func(c *fiber.Ctx) error {
		if !isSupersetServiceRequest(c.Params("*")) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "operation Superset is service-only; use /superset/public/ for UI",
			})
		}
		return proxySuperset(c, cfg, sessions, codec, cfg.SupersetOperationURL, "/api/superset/operation", "/api/superset/operation", "operation")
	})

	app.All("/superset/public/*", func(c *fiber.Ctx) error {
		if isSupersetServiceRequest(c.Params("*")) {
			return proxySuperset(c, cfg, sessions, codec, cfg.SupersetOperationURL, "/api/superset/operation", "/superset/public", "operation")
		}
		return proxySuperset(c, cfg, sessions, codec, cfg.SupersetPublicURL, "/superset/public", "/superset/public", "public")
	})

	app.Post("/sessions", func(c *fiber.Ctx) error {
		var req struct {
			UserID string `json:"userId"`
			Zone   string `json:"zone"`
		}
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		if req.UserID == "" {
			req.UserID = "local-admin"
		}
		if req.Zone == "" {
			req.Zone = "public"
		}

		now := time.Now().UTC()
		doc := session{
			ID:        now.Format("20060102150405.000000000"),
			UserID:    req.UserID,
			Zone:      req.Zone,
			CreatedAt: now,
			ExpiresAt: now.Add(8 * time.Hour),
		}

		insertCtx, insertCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer insertCancel()

		if _, err := sessions.InsertOne(insertCtx, bson.M{
			"_id":       doc.ID,
			"userId":    doc.UserID,
			"zone":      doc.Zone,
			"createdAt": doc.CreatedAt,
			"expiresAt": doc.ExpiresAt,
		}); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.Status(fiber.StatusCreated).JSON(doc)
	})

	log.Fatal(app.Listen(":" + cfg.Port))
}

// proxySuperset هسته اصلی ارسال requestهای Superset است.
// این تابع token را از request یا session داخلی پیدا می‌کند، برای zone عملیات Token Exchange انجام می‌دهد،
// header Authorization را روی request می‌گذارد، target واقعی Superset را می‌سازد و response/redirect را اصلاح می‌کند.
func proxySuperset(c *fiber.Ctx, cfg config, sessions *mongo.Collection, codec *tokenCodec, upstreamBaseURL string, upstreamPrefix string, publicPrefix string, zone string) error {
	token := bearerToken(c)
	tokenSource := "request"
	tokenExchanged := false
	if token == "" {
		tokenSource = "session"
		sessionToken, err := sessionAccessToken(c, cfg, sessions, codec)
		if err != nil && zone == "operation" && env("ALLOW_SUPERSET_PROXY_WITHOUT_TOKEN", "false") != "true" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing or expired Super App session",
			})
		}
		token = sessionToken
	}
	if token != "" && zone == "operation" && cfg.OperationTokenExchangeEnabled {
		operationToken, err := exchangeOperationAccessToken(cfg, token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "operation token exchange failed",
			})
		}
		token = operationToken.AccessToken
		tokenExchanged = true
	}
	if token == "" && env("ALLOW_SUPERSET_PROXY_WITHOUT_TOKEN", "false") != "true" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing Keycloak access token",
		})
	}
	if token != "" {
		c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}

	wildcard := c.Params("*")
	if strings.HasSuffix(c.Path(), "/") && wildcard != "" && !strings.HasSuffix(wildcard, "/") {
		wildcard += "/"
	}
	targetWithoutQuery := strings.TrimRight(upstreamBaseURL, "/") + strings.TrimRight(upstreamPrefix, "/") + "/" + wildcard
	target := targetWithoutQuery
	if query := string(c.Request().URI().QueryString()); query != "" {
		target += "?" + query
	}

	c.Request().Header.Set("X-BI-Engine-Proxy", "super-app-backend")
	if zone == "operation" {
		log.Printf(
			"superset_operation_proxy method=%s path=%s upstream=%s sso_token_forwarded=%t token_source=%s token_exchange=%t token_fingerprint=%s",
			c.Method(),
			c.Path(),
			targetWithoutQuery,
			token != "",
			tokenSource,
			tokenExchanged,
			tokenFingerprint(token),
		)
	}
	if err := proxy.Do(c, target); err != nil {
		return err
	}
	rewriteSupersetRedirect(c, upstreamBaseURL, upstreamPrefix, publicPrefix)
	return nil
}

// isSupersetServiceRequest تشخیص می‌دهد یک مسیر Superset از جنس service/API/data است یا UI/asset.
// اگر خروجی true باشد request باید به Superset Operation برود؛ اگر false باشد مسیر برای Superset Public باقی می‌ماند.
// این تابع مرز معماری Public UI و Operation API را در Go proxy enforce می‌کند.
func isSupersetServiceRequest(wildcard string) bool {
	path := "/" + strings.TrimLeft(strings.ToLower(strings.TrimSpace(wildcard)), "/")
	servicePrefixes := []string{
		"/api",
		"/api/",
		"/superset/explore_json",
		"/superset/results",
		"/superset/slice_json",
		"/superset/log",
		"/superset/csv",
		"/superset/excel",
		"/superset/sqllab",
		"/superset/queries",
		"/superset/sql_json",
		"/savedqueryviewapi/",
		"/sqllab/",
	}
	for _, prefix := range servicePrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// tokenFingerprint برای لاگ‌گیری امن از token استفاده می‌شود.
// این تابع هیچ‌وقت token خام را چاپ نمی‌کند و فقط یک hash کوتاه می‌سازد تا بتوان requestها را trace کرد.
func tokenFingerprint(token string) string {
	if token == "" {
		return "none"
	}
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])[:16]
}

type tokenCodec struct {
	aead cipher.AEAD
}

// newTokenCodec ابزار رمزنگاری tokenها را از روی secret محیطی می‌سازد.
// هدف این است که access token و refresh token قبل از ذخیره در MongoDB با AES-GCM رمزنگاری شوند.
// اگر secret خالی یا مقدار پیش‌فرض ناامن باشد، backend عمدا start نمی‌شود.
func newTokenCodec(secret string) (*tokenCodec, error) {
	if secret == "" || strings.HasPrefix(secret, "change-me") {
		return nil, errors.New("SESSION_SECRET must be set to a strong non-default value")
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &tokenCodec{aead: aead}, nil
}

// encrypt یک مقدار حساس مثل access token یا refresh token را رمزنگاری می‌کند.
// برای هر مقدار یک nonce تصادفی ساخته می‌شود و خروجی به صورت base64 URL-safe ذخیره می‌شود.
func (codec *tokenCodec) encrypt(value string) (string, error) {
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := codec.aead.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// decrypt مقدار رمزنگاری‌شده ذخیره‌شده در MongoDB را به متن اصلی تبدیل می‌کند.
// این تابع هنگام خواندن token از session استفاده می‌شود تا Go proxy بتواند token معتبر را refresh یا exchange کند.
func (codec *tokenCodec) decrypt(value string) (string, error) {
	ciphertext, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	if len(ciphertext) < codec.aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}
	nonce := ciphertext[:codec.aead.NonceSize()]
	payload := ciphertext[codec.aead.NonceSize():]
	plaintext, err := codec.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// randomString رشته تصادفی امن می‌سازد.
// از این تابع برای state در OAuth و session id مرورگر استفاده می‌شود تا مقدارها قابل حدس زدن نباشند.
func randomString(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// sessionIDHash مقدار خام session id مرورگر را hash می‌کند.
// MongoDB فقط این hash را نگه می‌دارد، بنابراین در صورت leak شدن DB مقدار cookie واقعی افشا نمی‌شود.
func sessionIDHash(sessionID string) string {
	hash := sha256.Sum256([]byte(sessionID))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// loadSession session داخلی Super App را از روی cookie پیدا می‌کند.
// این تابع فقط sessionهایی را معتبر می‌داند که hash آن‌ها در MongoDB وجود داشته باشد و هنوز expire نشده باشند.
func loadSession(c *fiber.Ctx, sessions *mongo.Collection) (session, error) {
	sessionID := c.Cookies(sessionCookieName)
	if sessionID == "" {
		return session{}, errors.New("missing session cookie")
	}

	findCtx, findCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer findCancel()

	var doc session
	if err := sessions.FindOne(findCtx, bson.M{
		"_id":       sessionIDHash(sessionID),
		"expiresAt": bson.M{"$gt": time.Now().UTC()},
	}).Decode(&doc); err != nil {
		return session{}, err
	}
	return doc, nil
}

// sessionAccessToken access token عمومی Keycloak را از session داخلی برمی‌گرداند.
// اگر token هنوز معتبر باشد decrypt می‌شود؛ اگر نزدیک انقضا باشد با refresh token تمدید و دوباره در MongoDB ذخیره می‌شود.
// خروجی این تابع ورودی مرحله Token Exchange برای Superset Operation است.
func sessionAccessToken(c *fiber.Ctx, cfg config, sessions *mongo.Collection, codec *tokenCodec) (string, error) {
	doc, err := loadSession(c, sessions)
	if err != nil {
		return "", err
	}

	if time.Until(doc.TokenExpiresAt) > time.Minute {
		return codec.decrypt(doc.EncryptedAccessToken)
	}

	refreshToken, err := codec.decrypt(doc.EncryptedRefreshToken)
	if err != nil {
		return "", err
	}
	tokens, err := refreshAccessToken(cfg, refreshToken)
	if err != nil {
		return "", err
	}

	encryptedAccessToken, err := codec.encrypt(tokens.AccessToken)
	if err != nil {
		return "", err
	}
	encryptedRefreshToken := doc.EncryptedRefreshToken
	if tokens.RefreshToken != "" {
		encryptedRefreshToken, err = codec.encrypt(tokens.RefreshToken)
		if err != nil {
			return "", err
		}
	}

	updateCtx, updateCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer updateCancel()
	_, _ = sessions.UpdateOne(updateCtx, bson.M{"_id": doc.ID}, bson.M{
		"$set": bson.M{
			"encryptedAccessToken":  encryptedAccessToken,
			"encryptedRefreshToken": encryptedRefreshToken,
			"tokenExpiresAt":        time.Now().UTC().Add(time.Duration(tokens.ExpiresIn) * time.Second),
		},
	})

	return tokens.AccessToken, nil
}

// exchangeCode مرحله callback در Authorization Code Flow را کامل می‌کند.
// این تابع authorization code دریافتی از Keycloak عمومی را به access token و refresh token تبدیل می‌کند.
func exchangeCode(cfg config, code string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", cfg.redirectURI())
	return tokenRequest(cfg, values)
}

// refreshAccessToken با استفاده از refresh token عمومی، access token جدید می‌گیرد.
// این کار باعث می‌شود session کاربر در Go proxy بدون login مجدد تا زمان مجاز ادامه پیدا کند.
func refreshAccessToken(cfg config, refreshToken string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	return tokenRequest(cfg, values)
}

// tokenRequest درخواست عمومی token endpoint مربوط به Keycloak Public را ارسال می‌کند.
// این تابع برای exchange کردن authorization code و refresh کردن access token استفاده می‌شود.
// client id و client secret عمومی را به payload اضافه می‌کند و پاسخ Keycloak را validate می‌کند.
func tokenRequest(cfg config, values url.Values) (tokenResponse, error) {
	values.Set("client_id", cfg.KeycloakClientID)
	values.Set("client_secret", cfg.KeycloakClientSecret)

	req, err := http.NewRequest(http.MethodPost, cfg.keycloakRealmURL()+"/protocol/openid-connect/token", strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return tokenResponse{}, errors.New("Keycloak token exchange failed")
	}

	var tokens tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tokens); err != nil {
		return tokenResponse{}, err
	}
	if tokens.AccessToken == "" {
		return tokenResponse{}, errors.New("Keycloak did not return an access token")
	}
	return tokens, nil
}

// exchangeOperationAccessToken توکن عمومی کاربر را به توکن قابل قبول برای محیط Operation تبدیل می‌کند.
// این تابع grant type استاندارد Token Exchange را می‌سازد و subject token عمومی را به Keycloak عملیات می‌فرستد.
// خروجی آن access token عملیات است که به Superset Operation ارسال می‌شود.
func exchangeOperationAccessToken(cfg config, subjectToken string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	values.Set("subject_token", subjectToken)
	values.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	if cfg.OperationTokenExchangeRequestedTokenType != "" {
		values.Set("requested_token_type", cfg.OperationTokenExchangeRequestedTokenType)
	}
	if cfg.OperationTokenExchangeAudience != "" {
		values.Set("audience", cfg.OperationTokenExchangeAudience)
	}
	if cfg.OperationTokenExchangeRequestedIssuer != "" {
		values.Set("requested_issuer", cfg.OperationTokenExchangeRequestedIssuer)
	}
	return operationTokenRequest(cfg, values)
}

// operationTokenRequest payload Token Exchange را به token endpoint مربوط به Keycloak Operation ارسال می‌کند.
// این تابع از client محرمانه عملیات استفاده می‌کند و فقط وقتی پاسخ موفق و دارای access token باشد خروجی معتبر می‌دهد.
func operationTokenRequest(cfg config, values url.Values) (tokenResponse, error) {
	values.Set("client_id", cfg.OperationKeycloakClientID)
	values.Set("client_secret", cfg.OperationKeycloakClientSecret)

	req, err := http.NewRequest(http.MethodPost, cfg.operationKeycloakRealmURL()+"/protocol/openid-connect/token", strings.NewReader(values.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return tokenResponse{}, errors.New("operation Keycloak token exchange failed")
	}

	var tokens tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tokens); err != nil {
		return tokenResponse{}, err
	}
	if tokens.AccessToken == "" {
		return tokenResponse{}, errors.New("operation Keycloak did not return an access token")
	}
	return tokens, nil
}

// fetchUserInfo اطلاعات کاربر لاگین‌شده را از Keycloak عمومی می‌گیرد.
// خروجی آن برای ساخت session داخلی Super App و ثبت user id، username و email در MongoDB استفاده می‌شود.
func fetchUserInfo(cfg config, accessToken string) (userInfo, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.keycloakRealmURL()+"/protocol/openid-connect/userinfo", nil)
	if err != nil {
		return userInfo{}, err
	}
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+accessToken)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return userInfo{}, err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return userInfo{}, errors.New("Keycloak userinfo request failed")
	}

	var info userInfo
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return userInfo{}, err
	}
	if info.Subject == "" {
		return userInfo{}, errors.New("Keycloak userinfo did not return subject")
	}
	return info, nil
}

// keycloakRealmURL آدرس داخلی realm عمومی Keycloak را می‌سازد.
// backend از این URL برای token endpoint و userinfo در ارتباط server-to-server استفاده می‌کند.
func (cfg config) keycloakRealmURL() string {
	return strings.TrimRight(cfg.KeycloakBaseURL, "/") + "/realms/" + cfg.KeycloakRealm
}

// operationKeycloakRealmURL آدرس داخلی realm عملیات Keycloak را می‌سازد.
// این URL برای Token Exchange و ارتباط backend با Keycloak محیط Operation استفاده می‌شود.
func (cfg config) operationKeycloakRealmURL() string {
	return strings.TrimRight(cfg.OperationKeycloakBaseURL, "/") + "/realms/" + cfg.OperationKeycloakRealm
}

// keycloakPublicRealmURL آدرس قابل دسترس مرورگر برای realm عمومی Keycloak را می‌سازد.
// از این URL برای redirect کردن کاربر به صفحه login استفاده می‌شود.
func (cfg config) keycloakPublicRealmURL() string {
	return strings.TrimRight(cfg.KeycloakPublicURL, "/") + "/realms/" + cfg.KeycloakRealm
}

// redirectURI آدرس callback بک‌اند بعد از login در Keycloak را می‌سازد.
// مقدار ساخته‌شده باید با redirect URI تعریف‌شده در client عمومی Keycloak یکسان باشد.
func (cfg config) redirectURI() string {
	return strings.TrimRight(cfg.BackendPublicURL, "/") + "/auth/callback"
}

// bearerToken تلاش می‌کند token مستقیم موجود در request را استخراج کند.
// ابتدا header استاندارد Authorization بررسی می‌شود و سپس چند cookie شناخته‌شده خوانده می‌شود.
// در flow اصلی معماری معمولا token از session داخلی خوانده می‌شود، نه مستقیما از مرورگر.
func bearerToken(c *fiber.Ctx) string {
	header := c.Get(fiber.HeaderAuthorization)
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}

	for _, name := range []string{"KC_ACCESS_TOKEN", "KEYCLOAK_ACCESS_TOKEN", "access_token"} {
		if value := c.Cookies(name); value != "" {
			return value
		}
	}

	return ""
}

// safeReturnTo مسیر بازگشت بعد از login را validate می‌کند.
// فقط مسیرهای داخلی که با یک slash شروع می‌شوند قبول می‌شوند تا open redirect ایجاد نشود.
func safeReturnTo(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") || strings.Contains(value, "://") {
		return ""
	}
	return value
}

// rewriteSupersetRedirect مسیر redirectهای برگشتی از Superset را با مسیر proxy سازگار می‌کند.
// اگر Superset آدرس داخلی یا prefix خودش را در Location برگرداند، این تابع آن را به prefix قابل استفاده مرورگر تبدیل می‌کند.
// هدف این است که کاربر هیچ‌وقت از مسیر Go proxy خارج نشود.
func rewriteSupersetRedirect(c *fiber.Ctx, upstreamBaseURL string, upstreamPrefix string, publicPrefix string) {
	location := c.Response().Header.Peek(fiber.HeaderLocation)
	if len(location) == 0 {
		return
	}

	value := string(location)
	upstream := strings.TrimRight(upstreamBaseURL, "/")
	upstreamPath := strings.TrimRight(upstreamPrefix, "/")
	prefix := strings.TrimRight(publicPrefix, "/")

	if strings.HasPrefix(value, upstream+"/") {
		redirectPath := strings.TrimPrefix(value, upstream)
		if strings.HasPrefix(redirectPath, upstreamPath+"/") || redirectPath == upstreamPath {
			redirectPath = strings.TrimPrefix(redirectPath, upstreamPath)
		}
		c.Response().Header.Set(fiber.HeaderLocation, prefix+redirectPath)
		return
	}
	if strings.HasPrefix(value, upstreamPath+"/") || value == upstreamPath {
		c.Response().Header.Set(fiber.HeaderLocation, prefix+strings.TrimPrefix(value, upstreamPath))
		return
	}
	if !strings.HasPrefix(value, "/") && !strings.Contains(value, "://") {
		relativePrefix := strings.TrimLeft(prefix, "/")
		if strings.HasPrefix(value, relativePrefix+"/") {
			c.Response().Header.Set(fiber.HeaderLocation, "/"+value)
			return
		}
		c.Response().Header.Set(fiber.HeaderLocation, prefix+"/"+strings.TrimLeft(value, "/"))
		return
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, prefix+"/") {
		c.Response().Header.Set(fiber.HeaderLocation, prefix+value)
	}
}

// loadConfig تمام تنظیمات runtime را از env می‌خواند و مقدارهای پیش‌فرض local را اعمال می‌کند.
// این تابع مرز تنظیمات Public Keycloak، Operation Keycloak، Supersetها، MongoDB و رفتار Token Exchange را مشخص می‌کند.
func loadConfig() config {
	operationRequestedTokenType := env(
		"OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_TOKEN_TYPE",
		"urn:ietf:params:oauth:token-type:access_token",
	)
	return config{
		Port:                                     env("PORT", "8090"),
		MongoURI:                                 env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:                            env("MONGO_DATABASE", "bi_engine_platform"),
		SessionSecret:                            env("SESSION_SECRET", ""),
		CookieSecure:                             env("COOKIE_SECURE", "false") == "true",
		BackendPublicURL:                         env("BACKEND_PUBLIC_URL", "http://localhost:8080/api"),
		KeycloakBaseURL:                          env("KEYCLOAK_BASE_URL", "http://localhost:8081"),
		KeycloakPublicURL:                        env("KEYCLOAK_PUBLIC_URL", env("KEYCLOAK_BASE_URL", "http://localhost:8081")),
		KeycloakRealm:                            env("KEYCLOAK_REALM", "bi-engine"),
		KeycloakClientID:                         env("KEYCLOAK_CLIENT_ID", "super-app"),
		KeycloakClientSecret:                     env("KEYCLOAK_CLIENT_SECRET", ""),
		OperationKeycloakBaseURL:                 env("OPERATION_KEYCLOAK_BASE_URL", env("KEYCLOAK_BASE_URL", "http://localhost:8081")),
		OperationKeycloakRealm:                   env("OPERATION_KEYCLOAK_REALM", env("KEYCLOAK_REALM", "bi-engine")),
		OperationKeycloakClientID:                env("OPERATION_KEYCLOAK_CLIENT_ID", env("KEYCLOAK_CLIENT_ID", "super-app")),
		OperationKeycloakClientSecret:            env("OPERATION_KEYCLOAK_CLIENT_SECRET", env("KEYCLOAK_CLIENT_SECRET", "")),
		OperationTokenExchangeEnabled:            env("OPERATION_KEYCLOAK_TOKEN_EXCHANGE_ENABLED", "true") == "true",
		OperationTokenExchangeAudience:           env("OPERATION_KEYCLOAK_TOKEN_EXCHANGE_AUDIENCE", ""),
		OperationTokenExchangeRequestedIssuer:    env("OPERATION_KEYCLOAK_TOKEN_EXCHANGE_REQUESTED_ISSUER", ""),
		OperationTokenExchangeRequestedTokenType: operationRequestedTokenType,
		SupersetPublicURL:                        env("SUPERSET_PUBLIC_URL", "http://localhost:8088"),
		SupersetOperationURL:                     env("SUPERSET_OPERATION_URL", "http://localhost:8089"),
	}
}

// env یک helper کوچک برای خواندن متغیر محیطی است.
// اگر مقدار env خالی باشد، fallback برگردانده می‌شود تا compose local بدون تنظیم همه متغیرها اجرا شود.
func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

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
	Port                 string `json:"port"`
	MongoURI             string `json:"-"`
	MongoDatabase        string `json:"mongoDatabase"`
	SessionSecret        string `json:"-"`
	CookieSecure         bool   `json:"-"`
	BackendPublicURL     string `json:"backendPublicUrl"`
	KeycloakBaseURL      string `json:"keycloakBaseUrl"`
	KeycloakPublicURL    string `json:"keycloakPublicUrl"`
	KeycloakRealm        string `json:"keycloakRealm"`
	KeycloakClientID     string `json:"keycloakClientId"`
	KeycloakClientSecret string `json:"-"`
	SupersetPublicURL    string `json:"supersetPublicUrl"`
	SupersetOperationURL string `json:"supersetOperationUrl"`
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
	sessionCookieName = "BI_ENGINE_SESSION"
	stateCookieName   = "BI_ENGINE_OAUTH_STATE"
)

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
				{"name": "public", "url": cfg.SupersetPublicURL},
				{"name": "operation", "url": "/api/superset/operation/"},
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

		return c.Redirect("/superset-mfe/", fiber.StatusFound)
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
		token := bearerToken(c)
		if token == "" {
			sessionToken, err := sessionAccessToken(c, cfg, sessions, codec)
			if err != nil && env("ALLOW_SUPERSET_PROXY_WITHOUT_TOKEN", "false") != "true" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "missing or expired Super App session",
				})
			}
			token = sessionToken
		}
		if token == "" && env("ALLOW_SUPERSET_PROXY_WITHOUT_TOKEN", "false") != "true" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "missing Keycloak access token",
			})
		}
		if token != "" {
			c.Request().Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
		}

		target := strings.TrimRight(cfg.SupersetOperationURL, "/") + "/" + c.Params("*")
		if query := string(c.Request().URI().QueryString()); query != "" {
			target += "?" + query
		}

		c.Request().Header.Set("X-BI-Engine-Proxy", "super-app-backend")
		c.Request().Header.Set("X-Forwarded-Prefix", "/api/superset/operation")
		return proxy.Do(c, target)
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

type tokenCodec struct {
	aead cipher.AEAD
}

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

func (codec *tokenCodec) encrypt(value string) (string, error) {
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := codec.aead.Seal(nonce, nonce, []byte(value), nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

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

func randomString(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func sessionIDHash(sessionID string) string {
	hash := sha256.Sum256([]byte(sessionID))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

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

func exchangeCode(cfg config, code string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", cfg.redirectURI())
	return tokenRequest(cfg, values)
}

func refreshAccessToken(cfg config, refreshToken string) (tokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	return tokenRequest(cfg, values)
}

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

func (cfg config) keycloakRealmURL() string {
	return strings.TrimRight(cfg.KeycloakBaseURL, "/") + "/realms/" + cfg.KeycloakRealm
}

func (cfg config) keycloakPublicRealmURL() string {
	return strings.TrimRight(cfg.KeycloakPublicURL, "/") + "/realms/" + cfg.KeycloakRealm
}

func (cfg config) redirectURI() string {
	return strings.TrimRight(cfg.BackendPublicURL, "/") + "/auth/callback"
}

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

func loadConfig() config {
	return config{
		Port:                 env("PORT", "8090"),
		MongoURI:             env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:        env("MONGO_DATABASE", "bi_engine_platform"),
		SessionSecret:        env("SESSION_SECRET", ""),
		CookieSecure:         env("COOKIE_SECURE", "false") == "true",
		BackendPublicURL:     env("BACKEND_PUBLIC_URL", "http://localhost:8080/api"),
		KeycloakBaseURL:      env("KEYCLOAK_BASE_URL", "http://localhost:8081"),
		KeycloakPublicURL:    env("KEYCLOAK_PUBLIC_URL", env("KEYCLOAK_BASE_URL", "http://localhost:8081")),
		KeycloakRealm:        env("KEYCLOAK_REALM", "bi-engine"),
		KeycloakClientID:     env("KEYCLOAK_CLIENT_ID", "super-app"),
		KeycloakClientSecret: env("KEYCLOAK_CLIENT_SECRET", ""),
		SupersetPublicURL:    env("SUPERSET_PUBLIC_URL", "http://localhost:8088"),
		SupersetOperationURL: env("SUPERSET_OPERATION_URL", "http://localhost:8089"),
	}
}

func env(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

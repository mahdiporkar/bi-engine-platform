package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type config struct {
	Port                  string `json:"port"`
	MongoURI              string `json:"-"`
	MongoDatabase         string `json:"mongoDatabase"`
	KeycloakBaseURL       string `json:"keycloakBaseUrl"`
	KeycloakRealm         string `json:"keycloakRealm"`
	KeycloakClientID      string `json:"keycloakClientId"`
	SupersetPublicURL     string `json:"supersetPublicUrl"`
	SupersetOperationURL  string `json:"supersetOperationUrl"`
}

type session struct {
	ID        string    `bson:"_id" json:"id"`
	UserID    string    `bson:"userId" json:"userId"`
	Zone      string    `bson:"zone" json:"zone"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	ExpiresAt time.Time `bson:"expiresAt" json:"expiresAt"`
}

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
				{"name": "operation", "url": cfg.SupersetOperationURL},
			},
		})
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

func loadConfig() config {
	return config{
		Port:                 env("PORT", "8090"),
		MongoURI:             env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:        env("MONGO_DATABASE", "bi_engine_platform"),
		KeycloakBaseURL:      env("KEYCLOAK_BASE_URL", "http://localhost:8081"),
		KeycloakRealm:        env("KEYCLOAK_REALM", "bi-engine"),
		KeycloakClientID:     env("KEYCLOAK_CLIENT_ID", "super-app"),
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

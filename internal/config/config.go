package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Address, RedisAddress, RedisPassword, Origin string
	AdminEmail, AdminPassword                    string
	MongoURI, MongoDatabase                      string
	SecureCookies                                bool
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}
	c := Config{
		Address: env("HTTP_ADDR", "127.0.0.1:8080"), RedisAddress: env("REDIS_ADDR", "localhost:6380"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"), Origin: env("APP_ORIGIN", "http://localhost:3000"),
		MongoURI: os.Getenv("MONGODB_URI"), MongoDatabase: env("MONGODB_DATABASE", "seatbooking"),
		AdminEmail: os.Getenv("ADMIN_EMAIL"), AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}
	if c.MongoURI == "" {
		return c, fmt.Errorf("MONGODB_URI is required in .env or environment")
	}
	u, err := url.Parse(c.Origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return c, fmt.Errorf("APP_ORIGIN must be an origin such as http://localhost:3000")
	}
	c.SecureCookies = u.Scheme == "https"
	if !c.SecureCookies && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return c, fmt.Errorf("APP_ORIGIN must use HTTPS outside localhost")
	}
	if (c.AdminEmail == "") != (c.AdminPassword == "") {
		return c, fmt.Errorf("set both ADMIN_EMAIL and ADMIN_PASSWORD to bootstrap an admin")
	}
	return c, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

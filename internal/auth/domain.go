package auth

import (
	"context"
	"time"
)

const (
	RoleUser   = "USER"
	RoleAdmin  = "ADMIN"
	AccessTTL  = 15 * time.Minute
	RefreshTTL = 7 * 24 * time.Hour
)

type User struct {
	ID    string `json:"id" bson:"id"`
	Name  string `json:"name" bson:"name"`
	Email string `json:"email" bson:"email"`
	Role  string `json:"role" bson:"role"`
}

type Credentials struct {
	User         User   `json:"user" bson:"user"`
	PasswordHash string `json:"-" bson:"password_hash"`
}

type Tokens struct{ Access, Refresh string }

type UserStore interface {
	CreateUser(context.Context, Credentials) error
	UserByEmail(context.Context, string) (Credentials, error)
	UserByID(context.Context, string) (User, error)
}

type SessionStore interface {
	CreateSession(context.Context, string, string, string, string) error
	SessionUser(context.Context, string) (string, error)
	AccessUser(context.Context, string, string) (string, error)
	Rotate(context.Context, string, string, string, string) (string, error)
	Revoke(context.Context, string, string) error
}

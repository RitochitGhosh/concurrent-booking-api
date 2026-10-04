package auth

import (
	"context"
	"errors"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoStore struct{ users *mongo.Collection }

func NewMongoStore(db *mongo.Database) *MongoStore { return &MongoStore{users: db.Collection("users")} }
func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.users.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "user.email", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "user.id", Value: 1}}, Options: options.Index().SetUnique(true)},
	})
	return err
}
func (s *MongoStore) CreateUser(ctx context.Context, c Credentials) error {
	_, err := s.users.InsertOne(ctx, c)
	if mongo.IsDuplicateKeyError(err) {
		return platform.ErrConflict
	}
	return err
}
func (s *MongoStore) credentials(ctx context.Context, filter bson.M) (Credentials, error) {
	var c Credentials
	err := s.users.FindOne(ctx, filter).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Credentials{}, platform.ErrNotFound
	}
	return c, err
}
func (s *MongoStore) UserByEmail(ctx context.Context, email string) (Credentials, error) {
	return s.credentials(ctx, bson.M{"user.email": email})
}
func (s *MongoStore) UserByID(ctx context.Context, id string) (User, error) {
	c, err := s.credentials(ctx, bson.M{"user.id": id})
	return c.User, err
}

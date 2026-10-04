package movie

import (
	"context"
	"errors"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoStore struct{ movies *mongo.Collection }

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{movies: db.Collection("movies")}
}
func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.movies.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "starts_at", Value: 1}}},
	})
	return err
}
func (s *MongoStore) Create(ctx context.Context, m Movie) error {
	_, err := s.movies.InsertOne(ctx, m)
	return err
}
func (s *MongoStore) Get(ctx context.Context, id string) (Movie, error) {
	var m Movie
	err := s.movies.FindOne(ctx, bson.M{"id": id}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Movie{}, platform.ErrNotFound
	}
	return m, err
}
func (s *MongoStore) List(ctx context.Context) ([]Movie, error) {
	cursor, err := s.movies.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "starts_at", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	movies := make([]Movie, 0)
	err = cursor.All(ctx, &movies)
	return movies, err
}

package booking

import (
	"context"
	"errors"
	"time"

	"github.com/RitochitGhosh/seatbooking-api/internal/platform"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// One document per screening/seat is the authoritative reservation state.
// Expiry is logical: reclaim expired holds atomically instead of waiting for a TTL sweep.
type MongoStore struct{ bookings *mongo.Collection }

func NewMongoStore(db *mongo.Database) *MongoStore {
	return &MongoStore{bookings: db.Collection("bookings")}
}
func (s *MongoStore) EnsureIndexes(ctx context.Context) error {
	_, err := s.bookings.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "movie_id", Value: 1}, {Key: "seat_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}
func (s *MongoStore) Hold(ctx context.Context, b Booking) (Booking, error) {
	now := time.Now().UTC()
	b.ID, b.Status, b.ExpiresAt = uuid.NewString(), "held", now.Add(defaultHoldTTL)
	filter := bson.M{"movie_id": b.MovieID, "seat_id": b.SeatID, "status": "held", "expires_at": bson.M{"$lte": now}}
	// If the seat is free, upsert inserts it. If a live hold or confirmation
	// already exists, the unique index rejects the competing upsert.
	var result Booking
	err := s.bookings.FindOneAndUpdate(ctx, filter, bson.M{"$set": b}, options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)).Decode(&result)
	if mongo.IsDuplicateKeyError(err) {
		return Booking{}, ErrSeatAlreadyBooked
	}
	return result, err
}
func (s *MongoStore) Confirm(ctx context.Context, movieID, seatID, id, userID string) (Booking, error) {
	filter := bson.M{"movie_id": movieID, "seat_id": seatID, "id": id, "user_id": userID, "status": "held", "expires_at": bson.M{"$gt": time.Now().UTC()}}
	update := bson.M{"$set": bson.M{"status": "confirmed"}, "$unset": bson.M{"expires_at": ""}}
	var result Booking
	err := s.bookings.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&result)
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return result, err
	}
	// A retry after a lost response returns the same confirmed reservation.
	err = s.bookings.FindOne(ctx, bson.M{"movie_id": movieID, "seat_id": seatID, "id": id}).Decode(&result)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Booking{}, platform.ErrNotFound
	}
	if err != nil {
		return Booking{}, err
	}
	if result.UserID != userID {
		return Booking{}, platform.ErrForbidden
	}
	if result.Status != "confirmed" {
		return Booking{}, platform.ErrNotFound
	}
	return result, nil
}

// Match the hold identity and status in the delete itself so release cannot
// delete a concurrent confirmation or a replacement hold after expiry.
func (s *MongoStore) Release(ctx context.Context, movieID, seatID, id, userID string) error {
	identity := bson.M{"movie_id": movieID, "seat_id": seatID, "id": id}
	filter := bson.M{"movie_id": movieID, "seat_id": seatID, "id": id, "user_id": userID, "status": "held"}
	result, err := s.bookings.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 1 {
		return nil
	}
	var b Booking
	err = s.bookings.FindOne(ctx, identity).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil
	} // Safe to retry a completed release.
	if err != nil {
		return err
	}
	if b.UserID != userID {
		return platform.ErrForbidden
	}
	return ErrHoldNotActive
}

func (s *MongoStore) List(ctx context.Context, movieID string) ([]Booking, error) {
	filter := bson.M{"movie_id": movieID, "$or": bson.A{bson.M{"status": "confirmed"}, bson.M{"status": "held", "expires_at": bson.M{"$gt": time.Now().UTC()}}}}
	cursor, err := s.bookings.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	bookings := make([]Booking, 0)
	err = cursor.All(ctx, &bookings)
	return bookings, err
}

package movie

import (
	"context"
	"time"
)

// A movie represents one screening. A later version can separate movies and shows.
type Movie struct {
	ID              string    `json:"id" bson:"id"`
	Title           string    `json:"title" bson:"title"`
	PosterURL       string    `json:"poster_url,omitempty" bson:"poster_url,omitempty"`
	Synopsis        string    `json:"synopsis" bson:"synopsis"`
	Genre           string    `json:"genre" bson:"genre"`
	DurationMinutes int       `json:"duration_minutes" bson:"duration_minutes"`
	StartsAt        time.Time `json:"starts_at" bson:"starts_at"`
	Rows            int       `json:"rows" bson:"rows"`
	SeatsPerRow     int       `json:"seats_per_row" bson:"seats_per_row"`
	CreatedAt       time.Time `json:"created_at" bson:"created_at"`
}

type Store interface {
	Create(context.Context, Movie) error
	Get(context.Context, string) (Movie, error)
	List(context.Context) ([]Movie, error)
}

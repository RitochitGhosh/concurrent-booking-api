package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, addr, password string) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{Addr: addr, Password: password, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second, ContextTimeoutEnabled: true})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect Redis: %w", err)
	}
	return client, nil
}

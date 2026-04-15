//go:build integration

package redis_test

import (
	"context"
	"testing"
	"time"

	cacherepo "kleido/internal/repository/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func testRedisCache(t *testing.T) *goredis.Client {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}

	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr: host + ":" + port.Port(),
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis ping: %v", err)
	}

	return rdb
}

type cachedValue struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestCacheSetAndGetJSON(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	key := "cache:test:user"
	value := cachedValue{Name: "Alice", Age: 30}
	ttl := 5 * time.Minute

	if err := repo.SetJSON(ctx, key, value, ttl); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}

	var dest cachedValue
	if err := repo.GetJSON(ctx, key, &dest); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	if dest.Name != value.Name {
		t.Errorf("Name: want %q, got %q", value.Name, dest.Name)
	}
	if dest.Age != value.Age {
		t.Errorf("Age: want %d, got %d", value.Age, dest.Age)
	}
}

func TestCacheGetJSON_NotFound(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	var dest cachedValue
	err := repo.GetJSON(ctx, "cache:nonexistent", &dest)
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
	if err != goredis.Nil {
		t.Errorf("expected redis.Nil, got: %v", err)
	}
}

func TestCacheDelete(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	key := "cache:delete:test"
	value := cachedValue{Name: "Bob", Age: 25}

	if err := repo.SetJSON(ctx, key, value, time.Hour); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}

	if err := repo.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var dest cachedValue
	err := repo.GetJSON(ctx, key, &dest)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestCacheDelete_MultipleKeys(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	keys := []string{"cache:multi:1", "cache:multi:2", "cache:multi:3"}
	for _, key := range keys {
		if err := repo.SetJSON(ctx, key, cachedValue{Name: "test"}, time.Hour); err != nil {
			t.Fatalf("SetJSON(%s): %v", key, err)
		}
	}

	if err := repo.Delete(ctx, keys...); err != nil {
		t.Fatalf("Delete multiple: %v", err)
	}

	for _, key := range keys {
		exists, err := repo.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists(%s): %v", key, err)
		}
		if exists {
			t.Errorf("key %s: expected not to exist after delete", key)
		}
	}
}

func TestCacheExists(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	key := "cache:exists:test"

	exists, err := repo.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists (before set): %v", err)
	}
	if exists {
		t.Error("expected false for non-existent key")
	}

	if err := repo.SetJSON(ctx, key, cachedValue{Name: "Carol", Age: 35}, time.Hour); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}

	exists, err = repo.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists (after set): %v", err)
	}
	if !exists {
		t.Error("expected true for existing key")
	}
}

func TestCacheSetJSON_Overwrite(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	key := "cache:overwrite"

	original := cachedValue{Name: "Original", Age: 10}
	if err := repo.SetJSON(ctx, key, original, time.Hour); err != nil {
		t.Fatalf("SetJSON (original): %v", err)
	}

	updated := cachedValue{Name: "Updated", Age: 20}
	if err := repo.SetJSON(ctx, key, updated, time.Hour); err != nil {
		t.Fatalf("SetJSON (updated): %v", err)
	}

	var dest cachedValue
	if err := repo.GetJSON(ctx, key, &dest); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	if dest.Name != updated.Name {
		t.Errorf("Name: want %q, got %q", updated.Name, dest.Name)
	}
	if dest.Age != updated.Age {
		t.Errorf("Age: want %d, got %d", updated.Age, dest.Age)
	}
}

func TestCacheSetJSON_InvalidJSON(t *testing.T) {
	rdb := testRedisCache(t)
	repo := cacherepo.NewCacheRepo(rdb)
	ctx := context.Background()

	type unmarshable struct {
		Chan chan int `json:"chan"`
	}

	err := repo.SetJSON(ctx, "cache:invalid", unmarshable{Chan: make(chan int)}, time.Hour)
	if err == nil {
		t.Error("expected error for non-marshallable value, got nil")
	}
}

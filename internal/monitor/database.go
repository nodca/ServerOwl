package monitor

import (
	"context"
	"database/sql"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

var (
	pgPool     *sql.DB
	pgPoolOnce sync.Once
	pgDSN      string // 保存 DSN

	redisClient     *redis.Client
	redisClientOnce sync.Once
	redisAddr       string // 保存地址
)

type DBCheckResult struct {
	Name    string // "postgres" 或 "redis"
	Healthy bool
	Latency time.Duration
	Error   string
}

func getPostgresPool(dsn string) (*sql.DB, error) {
	var err error
	pgPoolOnce.Do(func() {
		pgDSN = dsn
		pgPool, err = sql.Open("postgres", dsn)
		if err == nil {
			pgPool.SetMaxOpenConns(5)
			pgPool.SetMaxIdleConns(2)
			pgPool.SetConnMaxLifetime(time.Hour)
		}
	})

	// DSN 变了，需要重新创建
	if dsn != pgDSN && pgPool != nil {
		pgPool.Close()
		pgPool, err = sql.Open("postgres", dsn)
		pgDSN = dsn
	}

	return pgPool, err
}

// 检查 PostgreSQL
func CheckPostgres(dsn string, timeout time.Duration) DBCheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	now := time.Now()

	db, err := getPostgresPool(dsn)
	if err != nil {
		return DBCheckResult{
			Name:    "postgres",
			Healthy: false,
			Latency: time.Since(now),
			Error:   err.Error(),
		}
	}
	if db == nil {
		return DBCheckResult{
			Name:    "postgres",
			Healthy: false,
			Latency: time.Since(now),
			Error:   "sql.Open returned nil *sql.DB",
		}
	}

	err = db.PingContext(ctx)
	if err != nil {
		return DBCheckResult{
			Name:    "postgres",
			Healthy: false,
			Latency: time.Since(now),
			Error:   err.Error(),
		}
	}
	return DBCheckResult{
		Name:    "postgres",
		Healthy: true,
		Latency: time.Since(now),
		Error:   "",
	}

}

func getRedisClient(addr string, password string) *redis.Client {
	redisClientOnce.Do(func() {
		redisAddr = addr
		redisClient = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password})
	})
	//地址变了,重新加载
	if redisAddr != addr {
		redisClient.Close()
		redisAddr = addr
		redisClient = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: password,
		})
	}
	return redisClient
}

// 检查 Redis
func CheckRedis(addr, password string, timeout time.Duration) DBCheckResult {
	rdb := getRedisClient(addr, password)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	now := time.Now()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return DBCheckResult{
			Name:    "redis",
			Healthy: false,
			Latency: time.Since(now),
			Error:   err.Error(),
		}
	}
	return DBCheckResult{
		Name:    "redis",
		Healthy: true,
		Latency: time.Since(now),
		Error:   "",
	}
}

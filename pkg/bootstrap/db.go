package bootstrap

import (
	"database/sql"

	"github.com/dgraph-io/badger/v4"
	"github.com/redis/go-redis/v9"
	bolt "go.etcd.io/bbolt"
)

type Database struct {
	RedisClient *redis.Client
	SQLDB       *sql.DB
	BadgerDB    *badger.DB
	BoltDB      *bolt.DB
}

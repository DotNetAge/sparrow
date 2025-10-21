package projection

import (
	"testing"

	"github.com/DotNetAge/sparrow/pkg/config"
	"github.com/DotNetAge/sparrow/pkg/logger"
	"github.com/nats-io/nats.go"
)

func TestJetStreamIndexer(t *testing.T) {
	conn, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		t.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer conn.Close()

	logger, _ := logger.NewLogger(&config.LogConfig{
		Mode:  "dev",
		Level: "debug",
	})

	indexer := NewJetStreamIndexer(conn, "AuthService", logger)
	aggregateIDs, err := indexer.GetAllAggregateIDs("User")
	if err != nil {
		t.Fatalf("Failed to get all aggregate IDs: %v", err)
	}

	if len(aggregateIDs) == 0 {
		t.Fatalf("No aggregate IDs found")
	}

	// reader := messaging.NewJetStreamReader(conn, "AuthService", "User", logger)
	// reader.Replay(context.Background(), aggregateIDs[0], nil)
}

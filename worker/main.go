package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	kingpin "github.com/alecthomas/kingpin/v2"

	"github.com/IBM/sarama"
)

var (
	brokerList = kingpin.Flag("brokerList", "List of brokers to connect").Default("kafka:9092").Strings()
	topic      = kingpin.Flag("topic", "Topic name").Default("votes").String()
)

const (
	consumerGroup = "worker-group"
	host          = "postgresql"
	port          = 5432
	user          = "okteto"
	password      = "okteto"
	dbname        = "votes"
)

func main() {
	kingpin.Parse()

	db := openDatabase()
	defer db.Close()

	pingDatabase(db)

	createTableStmt := `CREATE TABLE IF NOT EXISTS votes (id VARCHAR(255) NOT NULL UNIQUE, vote VARCHAR(255) NOT NULL)`
	if _, err := db.Exec(createTableStmt); err != nil {
		log.Panic(err)
	}

	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	config.Consumer.Return.Errors = true

	brokers := *brokerList
	handler := &voteConsumer{db: db}

	fmt.Printf("Connecting to Kafka consumer group '%s' on topic '%s'...\n", consumerGroup, *topic)

	group, err := newConsumerGroup(brokers, consumerGroup, config)
	if err != nil {
		log.Panicf("Error creating consumer group: %v", err)
	}
	defer group.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Track errors
	go func() {
		for err := range group.Errors() {
			fmt.Printf("Consumer group error: %v\n", err)
		}
	}()

	// Consume in a goroutine
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if err := group.Consume(ctx, []string{*topic}, handler); err != nil {
				fmt.Printf("Error from consumer: %v\n", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()

	fmt.Println("Worker is running. Press Ctrl+C to exit.")

	// Wait for interrupt signal
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	fmt.Println("Interrupt received, shutting down...")
	cancel()
	wg.Wait()
	fmt.Println("Worker stopped.")
}

// newConsumerGroup retries connecting to Kafka with exponential backoff.
// Retry pattern: starts at 1s, doubles each attempt, caps at 30s.
func newConsumerGroup(brokers []string, group string, config *sarama.Config) (sarama.ConsumerGroup, error) {
	fmt.Println("Waiting for kafka...")
	delay := time.Second
	const maxDelay = 30 * time.Second
	for {
		cg, err := sarama.NewConsumerGroup(brokers, group, config)
		if err == nil {
			fmt.Println("Kafka connected!")
			return cg, nil
		}
		fmt.Printf("Kafka not ready, retrying in %s: %v\n", delay, err)
		time.Sleep(delay)
		if delay < maxDelay {
			delay *= 2
		}
	}
}

// voteConsumer implements sarama.ConsumerGroupHandler.
type voteConsumer struct {
	db *sql.DB
}

func (v *voteConsumer) Setup(sarama.ConsumerGroupSession) error {
	fmt.Println("Consumer group session started — partitions assigned")
	return nil
}

func (v *voteConsumer) Cleanup(sarama.ConsumerGroupSession) error {
	fmt.Println("Consumer group session ended — partitions revoked")
	return nil
}

func (v *voteConsumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		voterID := string(msg.Key)
		vote := string(msg.Value)

		fmt.Printf("Received message: partition=%d offset=%d voter=%s vote=%s\n",
			msg.Partition, msg.Offset, voterID, vote)

		insertStmt := `INSERT INTO "votes"("id", "vote") VALUES($1, $2) ON CONFLICT(id) DO UPDATE SET vote = $2`
		if _, err := v.db.Exec(insertStmt, voterID, vote); err != nil {
			fmt.Printf("Error persisting vote: %v\n", err)
		}

		session.MarkMessage(msg, "")
	}
	return nil
}

func openDatabase() *sql.DB {
	psqlconn := buildPostgresConnString(host, port, user, password, dbname)
	for {
		db, err := sql.Open("postgres", psqlconn)
		if err == nil {
			return db
		}
	}
}

func buildPostgresConnString(host string, port int, user, password, dbname string) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
}

// pingDatabase retries the DB connection with exponential backoff.
// Retry pattern: starts at 1s, doubles each attempt, caps at 30s.
func pingDatabase(db *sql.DB) {
	fmt.Println("Waiting for postgresql...")
	delay := time.Second
	const maxDelay = 30 * time.Second
	for {
		if err := db.Ping(); err == nil {
			fmt.Println("Postgresql connected!")
			return
		}
		fmt.Printf("PostgreSQL not ready, retrying in %s\n", delay)
		time.Sleep(delay)
		if delay < maxDelay {
			delay *= 2
		}
	}
}

package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DBHost         string
	DBPort         string
	DBName         string
	DBUser         string
	DBPassword     string
	DBReplicaHosts string // comma-separated: host1:port1
	JWTSecret      string
	ServerAddr     string
	RedisAddr      string
	KafkaBrokers   string // comma-separated

	// Citus coordinator — separate connection used only for the dialog subsystem
	CitusHost     string
	CitusPort     string
	CitusDB       string
	CitusUser     string
	CitusPassword string
}

func Load() Config {
	return Config{
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBName:         getEnv("DB_NAME", "social"),
		DBUser:         getEnv("DB_USER", "social_user"),
		DBPassword:     getEnv("DB_PASSWORD", "social_pass"),
		DBReplicaHosts: getEnv("DB_REPLICA_HOSTS", ""),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-secret"),
		ServerAddr:     getEnv("SERVER_ADDR", ":8080"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:   getEnv("KAFKA_BROKERS", "localhost:9092"),

		CitusHost:     getEnv("CITUS_HOST", "localhost"),
		CitusPort:     getEnv("CITUS_PORT", "5435"),
		CitusDB:       getEnv("CITUS_DB", "social_dialogs"),
		CitusUser:     getEnv("CITUS_USER", "citus_user"),
		CitusPassword: getEnv("CITUS_PASSWORD", "citus_pass"),
	}
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
	)
}

// CitusDSN returns the connection string for the Citus coordinator.
func (c Config) CitusDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.CitusUser, c.CitusPassword, c.CitusHost, c.CitusPort, c.CitusDB,
	)
}

// ReplicaDSNs returns a list of individual DSNs for each replica.
// Falls back to master DSN if no replicas configured.
func (c Config) ReplicaDSNs() []string {
	if c.DBReplicaHosts == "" {
		return []string{c.DSN()}
	}
	hosts := strings.Split(c.DBReplicaHosts, ",")
	dsns := make([]string, 0, len(hosts))
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h != "" {
			dsns = append(dsns, fmt.Sprintf(
				"postgres://%s:%s@%s/%s?sslmode=disable",
				c.DBUser, c.DBPassword, h, c.DBName,
			))
		}
	}
	if len(dsns) == 0 {
		return []string{c.DSN()}
	}
	return dsns
}

func (c Config) KafkaBrokerList() []string {
	if c.KafkaBrokers == "" {
		return []string{"localhost:9092"}
	}
	return strings.Split(strings.ReplaceAll(c.KafkaBrokers, " ", ""), ",")
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

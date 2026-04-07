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
	DBReplicaHosts string // comma-separated: host1:port1,host2:port2
	JWTSecret      string
	ServerAddr     string
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
	}
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName,
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

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

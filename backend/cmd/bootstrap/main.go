package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/profora/myankafe-finance/backend/internal/config"
	"github.com/profora/myankafe-finance/backend/internal/ids"
	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func main() {
	cfg, err := config.Load()
	if err != nil { log.Fatal(err) }

	store, err := postgres.Open(context.Background(), cfg.DatabaseURL)
	if err != nil { log.Fatal(err) }
	defer store.Close()

	ctx := context.Background()
	username := getenv("BOOTSTRAP_USERNAME", "owner")
	display := getenv("BOOTSTRAP_DISPLAY_NAME", "Owner")
	email := os.Getenv("BOOTSTRAP_EMAIL")

	tx, err := store.Pool.Begin(ctx)
	if err != nil { log.Fatal(err) }
	defer tx.Rollback(ctx)

	var userID, publicID string
	err = tx.QueryRow(ctx, "SELECT id::text, public_id::text FROM users WHERE username=$1", username).Scan(&userID, &publicID)
	if err != nil {
		userID, _ = ids.UUIDv7()
		publicID, _ = ids.ULID()
		if _, err = tx.Exec(ctx, "INSERT INTO users(id,public_id,username,display_name,email) VALUES($1,$2,$3,$4,NULLIF($5,''))", userID, publicID, username, display, email); err != nil {
			log.Fatal(err)
		}
	}

	if strings.EqualFold(getenv("BOOTSTRAP_ENTITIES", "true"), "true") {
		type spec struct{ Code, Name, Kind string }
		for _, e := range []spec{
			{"MYANKAFE", "MyanKafe", "BUSINESS"},
			{"ROYAL_MASTERPIECE", "Royal Masterpiece", "BUSINESS"},
			{"PERSONAL", "Personal", "PERSONAL"},
		} {
			var entityID string
			err = tx.QueryRow(ctx, "SELECT id::text FROM entities WHERE code=$1", e.Code).Scan(&entityID)
			if err != nil {
				entityID, _ = ids.UUIDv7()
				entityPublic, _ := ids.ULID()
				if _, err = tx.Exec(ctx, "INSERT INTO entities(id,public_id,code,name,entity_type,functional_currency_code,timezone,created_by) VALUES($1,$2,$3,$4,$5,'MMK','Asia/Yangon',$6)", entityID, entityPublic, e.Code, e.Name, e.Kind, userID); err != nil {
					log.Fatal(err)
				}
			}
			roleLinkID, _ := ids.UUIDv7()
			if _, err = tx.Exec(ctx, "INSERT INTO user_entity_roles(id,user_id,entity_id,role_id,granted_by) VALUES($1,$2,$3,'00000000-0000-7000-8000-000000000001',$2) ON CONFLICT DO NOTHING", roleLinkID, userID, entityID); err != nil {
				log.Fatal(err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil { log.Fatal(err) }
	_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"owner_public_id": publicID, "username": username})
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" { return v }
	return d
}

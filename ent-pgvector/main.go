package main

import (
	"context"
	"fmt"
	"log"

	"ent-pgvector/ent"

	_ "github.com/lib/pq"
)

func main() {
	client, err := ent.Open("postgres", "postgres://postgres:postgres@localhost:5432/ent_pgvector?sslmode=disable")
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	defer client.Close()
	// Run the auto migration tool.
	if err := client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}

	u, err := client.User.
		Create().
		SetAge(30).
		Save(context.Background())
		//	SetVector([]float32{1.0, 2.0, 3.0}).
	if err != nil {
		log.Fatalf("failed creating user: %v", err)
	}
	fmt.Println(u)
}

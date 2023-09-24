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
	// client.User.Use(hook.On(func(next ent.Mutator) ent.Mutator {
	// 	return hook.UserFunc(func(ctx context.Context, m *ent.UserMutation) (ent.Value, error) {
	// 		// if description is set, or changed, we update the vector.
	// 		if m.Op == ent.OpCreate || m.DescriptionChanged() {
	// 			m.SetVector([]float32{1.0, 2.0, 3.0})

	// 		return next.Mutate(ctx, m)
	// 	})
	// }, ent.OpCreate|ent.OpUpdateOne))

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

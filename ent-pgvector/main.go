package main

import (
	"context"
	"fmt"
	"log"

	"ent-pgvector/ent"
	"ent-pgvector/ent/hook"

	"entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/embeddings/openai/openaichat"
	"github.com/tmc/langchaingo/llms/openai"
)

var embedder embeddings.Embedder

func main() {
	client, err := ent.Open("postgres", "postgres://postgres:postgres@localhost:5432/ent_pgvector?sslmode=disable")
	if err != nil {
		log.Fatalf("failed opening connection to postgres: %v", err)
	}
	defer client.Close()

	llm, err := openai.NewChat(openai.WithModel("text-embedding-ada-002"))
	if err != nil {
		log.Fatalf("failed opening connection to openai: %v", err)
	}
	oaic, err := openaichat.NewChatOpenAI(openaichat.WithClient(llm))
	if err != nil {
		log.Fatalf("failed opening connection to openai: %v", err)
	}
	embedder = oaic
	// embedder, err = eopenai.NewOpenAI(eopenai.WithClient(llm))
	// if err != nil {
	// 	log.Fatalf("failed opening connection to openai: %v", err)
	// }

	// Run the auto migration tool.
	if err := client.Schema.Create(context.Background()); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	client.User.Use(hook.On(func(next ent.Mutator) ent.Mutator {
		return hook.UserFunc(func(ctx context.Context, m *ent.UserMutation) (ent.Value, error) {
			// if description is set, or changed, we update the vector.
			description, ok := m.Description()
			if m.Op() == ent.OpCreate || ok {
				embedding, err := embed(description)
				if err != nil {
					log.Println(err)
				}
				m.SetEmbedding(pgvector.NewVector(embedding))
			}
			return next.Mutate(ctx, m)
		})
	}, ent.OpCreate|ent.OpUpdateOne))

	u, err := client.User.
		Create().
		SetAge(30).
		SetDescription("I am a purple dinosaur").
		Save(context.Background())
	if err != nil {
		log.Fatalf("failed creating user: %v", err)
	}
	fmt.Println(u)

	var uWDist []struct {
		*ent.User
		Distance float64
	}
	client = client.Debug()
	// Find the users with the highest similarity to the query.
	err = client.User.
		Query().
		Limit(4).
		Select("id", "name", "age", "description", "distance").
		Modify(func(q *sql.Selector) {
			q.AppendSelectAs(fmt.Sprintf("embeddings <-> '%s' as distance", u.Embedding), "distance")
		}).
		Scan(context.Background(), &uWDist)
		//All(context.Background()).Scan(&uWDist)

	// Modify(func(q *sql.Selector) {
	// 	// q.Where(sql.Raw("embeddings <=> ? < 0.5", u.Embedding))
	// 	q.AppendSelectExpr(sql.Raw(fmt.Sprintf("embeddings <-> '%s' as distance", u.Embedding)))
	// }).

	fmt.Printf("got %d users\n", len(uWDist))
	for _, u := range uWDist {
		fmt.Println(u)
	}
}

func embed(description string) ([]float32, error) {
	return embedder.EmbedQuery(context.Background(), description)
}

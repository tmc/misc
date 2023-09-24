package main

import (
	"context"
	"fmt"
	"log"

	"ent-pgvector/ent"
	"ent-pgvector/ent/hook"

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
		SetDescription("I am a user").
		Save(context.Background())
		//	SetVector([]float32{1.0, 2.0, 3.0}).
	if err != nil {
		log.Fatalf("failed creating user: %v", err)
	}
	fmt.Println(u)
}

func embed(description string) ([]float32, error) {
	r, err := embedder.EmbedQuery(context.Background(), description)
	if err != nil {
		return nil, err
	}
	result := make([]float32, len(r))
	for i, v := range r {
		result[i] = float32(v)
	}
	return result, nil
}

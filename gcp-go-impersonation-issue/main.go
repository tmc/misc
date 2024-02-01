package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/auth/impersonate"
)

func main() {
	fmt.Println(getIAPToken("https://foobar.com"))
}

func getIAPToken(audience string) (string, error) {
	ctx := context.Background()
	tp, err := impersonate.NewIDTokenProvider(&impersonate.IDTokenOptions{
		Audience: audience,
		//Audience:        "http://example.com/",
		TargetPrincipal: "github-actions@virta-eng-dev.iam.gserviceaccount.com",
		IncludeEmail:    true,
		// Optionally supply delegates.
		// Delegates: []string{"bar@project-id.iam.gserviceaccount.com"},
	})
	if err != nil {
		return "", fmt.Errorf("issue creating token provider: %w", err)
	}
	tok, err := tp.Token(ctx)
	if err != nil {
		return "", fmt.Errorf("issue generating token: %w", err)
	}
	return tok.Value, nil
}

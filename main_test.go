package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sethvargo/go-envconfig"
)

func TestFetchHtml(t *testing.T) {
	want := "<h1>OK</h1>"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte(want))
	}))

	defer server.Close()

	resp, err := fetchHtml(server.URL)

	if err != nil {
		t.Fatal(err)
	}

	if resp != want {
		t.Fatalf("'%v' doesn't match expected value: %v", resp, want)
	}
}

func TestConfig(t *testing.T) {
	ctx := context.Background()

	expectedJwtValue := "JWTSECRET"
	expectedApiValue := "GOOGLEAPIKEY"

	lookuper := envconfig.MapLookuper(map[string]string{
		"SUPABASE_JWT_SECRET": expectedJwtValue,
		"GOOGLE_AI_APIKEY":    expectedApiValue,
	})

	var config Config

	envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   &config,
		Lookuper: lookuper,
	})

	if config.Supabase.JwtSecret != expectedJwtValue || config.GoogleAI.APIKey != expectedApiValue {
		t.Fatalf("config doesn't match expected values")
	}
}

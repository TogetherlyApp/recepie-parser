package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Supabase struct {
		JwtSecret string `env:"SUPABASE_JWT_SECRET"`
	}
	GoogleAI struct {
		APIKey string `env:"GOOGLE_AI_APIKEY"`
	}
	Server struct {
		Host string `env:"HOST, default=0.0.0.0"`
		Port string `env:"PORT, default=8080"`
	}
}

var bm *bluemonday.Policy

func main() {
	bm = bluemonday.UGCPolicy()

	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	var c Config

	if err := envconfig.Process(ctx, &c); err != nil {
		log.Fatal(err)
	}

	handler, err := newServer(&c)
	if err != nil {
		return err
	}

	handler.logf("listening on http://%v:%v", c.Server.Host, c.Server.Port)

	s := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf("%s:%s", c.Server.Host, c.Server.Port),
		ReadTimeout:  time.Second * 10,
		WriteTimeout: time.Second * 30,
	}

	errorChannel := make(chan error, 1)
	go func() {
		errorChannel <- s.ListenAndServe()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	select {
	case err := <-errorChannel:
		log.Printf("failed to serve: %v", err)
	case sig := <-sigs:
		log.Printf("terminating: %v", sig)
	}

	return s.Shutdown(ctx)

}

func fetchHtml(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http status is not a success: %v", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func verifyToken(tokenString string, jwtKey string) error {
	tokenBytes := []byte(tokenString)
	_, err := jwt.Parse(tokenBytes, jwt.WithKey(jwa.HS256(), []byte(jwtKey)))
	if err != nil {
		return err
	}
	return nil
}

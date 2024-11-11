package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type server struct {
	config   Config
	serveMux http.ServeMux
	logf     func(f string, v ...interface{})
}

type RecepieParams struct {
	Url string `json:"url"`
}

func newServer(config *Config) (*server, error) {
	server := &server{
		logf:   log.Printf,
		config: *config,
	}

	server.registerRoutes()

	return server, nil
}

func (s *server) registerRoutes() {
	s.serveMux.HandleFunc("/recepie", s.recepieHandler)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.serveMux.ServeHTTP(w, r)
}

func (s *server) initializeGenAiClient() (*genai.Client, error) {
	ctx := context.Background()

	client, err := genai.NewClient(ctx, option.WithAPIKey(s.config.GoogleAI.APIKey))
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
		return nil, err
	}

	return client, nil
}

func (s *server) initializeGenAi(client *genai.Client) (*genai.GenerativeModel, error) {
	model := client.GenerativeModel("gemini-1.5-flash")

	model.SetTemperature(1)
	model.SetTopK(40)
	model.SetTopP(0.95)
	model.SetMaxOutputTokens(8192)
	model.ResponseMIMEType = "application/json"
	model.ResponseSchema = &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"ingredients": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type:     genai.TypeObject,
					Required: []string{"name", "amount"},
					Properties: map[string]*genai.Schema{
						"name": {
							Type: genai.TypeString,
						},
						"amount": {
							Type: genai.TypeString,
						},
					},
				},
			},
		},
	}

	return model, nil
}

func (s *server) recepieHandler(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	tokenSplit := strings.Split(authHeader, "Bearer ")

	if len(tokenSplit) != 2 {
		s.logf("invalid auth header provided")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := tokenSplit[1]

	err := verifyToken(token, s.config.Supabase.JwtSecret)

	if err != nil {
		s.logf("error in authorization: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var params RecepieParams

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&params); err != nil {
		s.logf("error in decoding recepie params: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	html, err := fetchHtml(params.Url)
	if err != nil {
		s.logf("error in fetching html: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	html = bm.Sanitize(html)

	client, err := s.initializeGenAiClient()
	if err != nil {
		s.logf("error in initializing gen ai client: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer client.Close()

	files := []string{s.uploadToGemini(ctx, client, html)}

	model, err := s.initializeGenAi(client)
	if err != nil {
		s.logf("error in creating the mode: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	session := model.StartChat()
	session.History = []*genai.Content{
		{
			Role: "user",
			Parts: []genai.Part{
				genai.FileData{URI: files[0]},
				genai.Text("Given this input HTML file, please extract the ingredients of the recepie and return them in a JSON format like:\n\n{\n \"ingredients\": [\n  { \"name\": \"sugar\", \"amount\": \"10g\" },\n  { \"name\": \"salt\", \"amount\": \"125g\" },\n  { \"name\": \"milk\", \"amount\": \"250ml\" },\n ]\n}\n\nMake sure to convert imperial units to metrical units and that the response is in German.\nIf a ingredient is mentioned multiple times, add the amount of them together.\nFurther remove additional information of a recepie like water being warm or that flour is needed for something specific. I just want to have the ingredient names."),
			},
		},
		{
			Role: "model",
			Parts: []genai.Part{
				genai.Text("```json\n{\"ingredients\": [{\"amount\": \"150 ml\", \"name\": \"Wasser, warm\"}, {\"amount\": \"100 g\", \"name\": \"Weizenmehl (Typ 405)\"}, {\"amount\": \"7 g\", \"name\": \"Trockenhefe\"}, {\"amount\": \"230 g\", \"name\": \"Weizenmehl (Typ 405)\"}, {\"amount\": \"30 ml\", \"name\": \"Pflanzensahne\"}, {\"amount\": \"20 g\", \"name\": \"Zucker\"}, {\"amount\": \"2 TL\", \"name\": \"Backmalz\"}, {\"amount\": \"1 TL\", \"name\": \"Ascorbinsäure\"}, {\"amount\": \"½ TL\", \"name\": \"Salz\"}, {\"amount\": \"100 g\", \"name\": \"Alsan Bio oder Alsan S\"}, {\"amount\": \"50 g\", \"name\": \"Alsan Bio oder Alsan S\"}, {\"amount\": \"4 EL\", \"name\": \"Pflanzensahne\"}, {\"amount\": \"n. B.\", \"name\": \"Blaumohn\"}, {\"amount\": \"n. B.\", \"name\": \"Sesam\"}, {\"amount\": \"n. B.\", \"name\": \"Sonnenblumenkerne\"}]}\n\n```"),
			},
		},
	}

	resp, err := session.SendMessage(ctx, genai.Text("Do"))
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("%v", part)))
		// json.NewEncoder(w).Encode(part)
	}
}

func (s *server) uploadToGemini(ctx context.Context, client *genai.Client, content string) string {
	r := strings.NewReader(content)
	options := genai.UploadFileOptions{
		DisplayName: "recepieHtml",
		MIMEType:    "text/plain",
	}

	fileData, err := client.UploadFile(ctx, "", r, &options)
	if err != nil {
		s.logf("Error uploading file: %v", err)
	}

	return fileData.URI
}

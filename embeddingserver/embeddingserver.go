package embeddingserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Embedding struct {
	Embedding []float32 `json:"embedding"`
	Aesthetic float32   `json:"aesthetic"`
}

type Task struct {
	Id    string `json:"id"`
	Image string `json:"image"`
	Text  string `json:"text"`
}

// Client represents the client that interacts with the image processing server.
type Client struct {
	ServerURL string // URL of the server endpoint
}

// NewClient creates a new instance of Client.
func NewClient(serverURL string) *Client {
	return &Client{
		ServerURL: serverURL,
	}
}

// SubmitImageTask submits an image for processing along with its ID.
func (c *Client) GetImageEmbedding(imageData []byte) (Embedding, error) {
	payload := Task{
		Image: base64.StdEncoding.EncodeToString(imageData),
	}

	return c.GetEmbedding(payload)
}

// SubmitTextTask submits text for processing along with its ID.
func (c *Client) GetTextEmbedding(text string) (Embedding, error) {
	payload := Task{
		Text: text,
	}

	return c.GetEmbedding(payload)
}

func (c *Client) GetEmbedding(payload Task) (Embedding, error) {
	payloadJSON, err := json.Marshal(payload)
	result := Embedding{}
	if err != nil {
		return result, fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.ServerURL+"/predict", bytes.NewBuffer(payloadJSON))
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return result, fmt.Errorf("server responded with status code: %d, response body: %s", resp.StatusCode, string(body))
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return result, fmt.Errorf("failed to decode server response: %w", err)
	}

	//fmt.Printf("Task submitted successfully with result: %+v\n", result)

	return result, nil
}

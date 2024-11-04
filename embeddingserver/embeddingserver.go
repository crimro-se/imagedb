package embeddingserver

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// a map (keyed by id string) of embedding results
// text embeddings always have an aesthetic of 0, for now.
type Embeddings map[string]struct {
	Embedding [768]float32 `json:"embedding"`
	Aesthetic float32      `json:"aesthetic"`
}

// Client represents the client that interacts with the image processing server.
type Client struct {
	ServerURL       string // URL of the server endpoint
	PendingRequests int
}

// NewClient creates a new instance of Client.
func New(serverURL string) *Client {
	return &Client{
		ServerURL: serverURL,
	}
}

// SubmitImageTask submits an image for processing along with its ID.
func (c *Client) SubmitImageTask(id string, imageData []byte) error {
	payload := map[string]interface{}{
		"id":    id,
		"image": base64.StdEncoding.EncodeToString(imageData),
	}

	return c.submitTask(payload)
}

// SubmitTextTask submits text for processing along with its ID.
func (c *Client) SubmitTextTask(id string, text string) error {
	payload := map[string]interface{}{
		"id":   id,
		"text": text,
	}

	return c.submitTask(payload)
}

// CollectResults collects the results from the server.
func (c *Client) CollectResults(results Embeddings) (Embeddings, error) {
	resp, err := http.Get(c.ServerURL + "/results")
	if err != nil {
		return nil, fmt.Errorf("failed to collect results: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}
	return results, nil
}

// submitTask sends a task to the server for processing.
func (c *Client) submitTask(payload map[string]interface{}) error {
	payloadJSON, err := json.Marshal([]map[string]interface{}{payload})
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.ServerURL+"/process", bytes.NewBuffer(payloadJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server responded with status code: %d, response body: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return fmt.Errorf("failed to decode server response: %w", err)
	}
	c.PendingRequests += 1
	fmt.Printf("Task submitted successfully with result: %+v\n", result)

	return nil
}

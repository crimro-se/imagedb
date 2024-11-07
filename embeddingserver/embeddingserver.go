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
func New(serverURL string) *Client {
	return &Client{
		ServerURL: serverURL,
	}
}

// SubmitImageTasks submits a list of image tasks for processing.
func (c *Client) SubmitImageTasks(imageIDs []string, imageDataList [][]byte) error {

	if len(imageIDs) != len(imageDataList) {
		return fmt.Errorf("imageIDs and data length missmatch")
	}

	if len(imageDataList) < 1 {
		return fmt.Errorf("nothing submitted")
	}

	payload := make([]Task, 0, len(imageIDs))
	for i, bytes := range imageDataList {
		payload = append(payload,
			Task{
				Id:    imageIDs[i],
				Image: base64.StdEncoding.EncodeToString(bytes),
			})
	}
	return c.submitTasks(payload)
}

// SubmitImageTask submits an image for processing along with its ID.
func (c *Client) SubmitImageTask(id string, imageData []byte) error {
	payload := []Task{{
		Id:    id,
		Image: base64.StdEncoding.EncodeToString(imageData),
	}}

	return c.submitTasks(payload)
}

// SubmitTextTask submits text for processing along with its ID.
func (c *Client) SubmitTextTask(id string, text string) error {
	payload := []Task{{
		Id:   id,
		Text: text,
	}}

	return c.submitTasks(payload)
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

// submitTasks sends tasks to the server for processing.
func (c *Client) submitTasks(payload []Task) error {
	payloadJSON, err := json.Marshal(payload)
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
	fmt.Printf("Task submitted successfully with result: %+v\n", result)

	return nil
}

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"testing"
)

var testimg_b64 string = `
iVBORw0KGgoAAAANSUhEUgAAAQAAAACAAgMAAACZ21+ZAAABhGlDQ1BJQ0MgcHJvZmlsZQAAKJF9
kT1Iw0AcxV9TpVIqDnbQ4pChOtlFRQSXWoUiVAi1QqsOJpd+QZOGpMXFUXAtOPixWHVwcdbVwVUQ
BD9AnB2cFF2kxP8lhRYxHhz34929x907QGhWmGb1xAFNr5npZELM5lbFwCuCiEDALIZlZhlzkpSC
5/i6h4+vdzGe5X3uz9Gv5i0G+ETiODPMGvEG8fRmzeC8TxxmJVklPiceN+mCxI9cV1x+41x0WOCZ
YTOTnicOE4vFLla6mJVMjXiKOKpqOuULWZdVzluctUqdte/JXxjK6yvLXKc5giQWsQQJIhTUUUYF
NcRo1UmxkKb9hIc/4vglcinkKoORYwFVaJAdP/gf/O7WKkxOuEmhBND7Ytsfo0BgF2g1bPv72LZb
J4D/GbjSO/5qE5j5JL3R0aJHwMA2cHHd0ZQ94HIHGHoyZFN2JD9NoVAA3s/om3LA4C0QXHN7a+/j
9AHIUFepG+DgEBgrUva6x7v7unv790y7vx/Op3LLMNEO0gAAAAlwSFlzAAAuIwAALiMBeKU/dgAA
AAd0SU1FB+gKGgMNCY3j5KUAAAAZdEVYdENvbW1lbnQAQ3JlYXRlZCB3aXRoIEdJTVBXgQ4XAAAA
CVBMVEUAAAAAAAD///+D3c/SAAAAAXRSTlMAQObYZgAAAAFiS0dEAmYLfGQAAABfSURBVGiB7c3B
CcAgEEVBm7AvD27/rQQ8hRBiDhLUzDsun9mUpEHlaFUAAPAIRPQeALYAOv0NiEsVAFgJyG1QABMD
LwIAAB8C52uZD0i3MwAAAAAAAAAAoB9gACBp+w67yiWwcnXERQAAAABJRU5ErkJggg==`

func loadTestImg(b64data string) (image.Image, error) {
	pngBytes, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	return img, err
}

// Struct to hold our JSON payload
type ImagePayload struct {
	Image string `json:"image"`
	Id    string `json:"id"`
}

func TestSendBase64Image(t *testing.T) {
	// Create JSON payload
	payload := []ImagePayload{{
		Image: testimg_b64,
		Id:    "example_image",
	}, {
		Image: testimg_b64,
		Id:    "example_image2",
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}, {
		Image: testimg_b64,
	}}

	// Marshal payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		t.Errorf("Failed to marshal JSON: %v", err)
		return
	}

	// Setup the request
	url := "http://localhost:5000/process"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		t.Errorf("Failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("Failed to send request: %v", err)
		return
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
		return
	}

	// Simple check for success (2XX OK)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Errorf("Expected status code 2XX but got %d. Response: %s", resp.StatusCode, string(respBody))
		return
	}

	fmt.Printf("Test successful. Server responded with: %s\n", string(respBody))
}

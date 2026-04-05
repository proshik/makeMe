package dictionary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Result holds transcription and audio URL for a word.
type Result struct {
	Transcription string
	AudioURL      string
}

// Client fetches pronunciation data from the Free Dictionary API.
// No API key required.
type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 5 * time.Second},
	}
}

type apiEntry struct {
	Phonetics []struct {
		Text  string `json:"text"`
		Audio string `json:"audio"`
	} `json:"phonetics"`
}

// Lookup returns transcription and audio URL for the given English word.
// Returns empty strings without error if the word is not found.
func (c *Client) Lookup(ctx context.Context, word string) (Result, error) {
	url := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%s", word)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Result{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var entries []apiEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return Result{}, fmt.Errorf("decode response: %w", err)
	}
	if len(entries) == 0 {
		return Result{}, nil
	}

	var result Result
	for _, p := range entries[0].Phonetics {
		if result.Transcription == "" && p.Text != "" {
			result.Transcription = p.Text
		}
		if result.AudioURL == "" && p.Audio != "" {
			result.AudioURL = p.Audio
		}
		if result.Transcription != "" && result.AudioURL != "" {
			break
		}
	}
	return result, nil
}

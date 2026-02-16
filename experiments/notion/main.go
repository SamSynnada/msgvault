package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const NotionAPI = "https://api.notion.com/v1"
const NotionVersion = "2024-06-15" // Latest stable version

type NotionPage struct {
	ID         string    `json:"id"`
	CreatedTime string    `json:"created_time"`
	LastEditedTime string `json:"last_edited_time"`
	CreatedBy  map[string]string `json:"created_by"`
	Properties map[string]interface{} `json:"properties"`
	Parent     map[string]interface{} `json:"parent"`
	Object     string `json:"object"`
}

type NotionBlock struct {
	ID    string      `json:"id"`
	Type  string      `json:"type"`
	Object string      `json:"object"`
	Paragraph *BlockContent `json:"paragraph,omitempty"`
	Heading1 *BlockContent `json:"heading_1,omitempty"`
	Heading2 *BlockContent `json:"heading_2,omitempty"`
	Heading3 *BlockContent `json:"heading_3,omitempty"`
	BulletedListItem *BlockContent `json:"bulleted_list_item,omitempty"`
	NumberedListItem *BlockContent `json:"numbered_list_item,omitempty"`
	Code *CodeBlock `json:"code,omitempty"`
	Image *ImageBlock `json:"image,omitempty"`
}

type BlockContent struct {
	RichText []RichText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
}

type CodeBlock struct {
	RichText []RichText `json:"rich_text"`
	Language string     `json:"language"`
	Caption  []RichText `json:"caption"`
}

type ImageBlock struct {
	Type     string      `json:"type"`
	File     *FileInfo   `json:"file,omitempty"`
	External *ExternalInfo `json:"external,omitempty"`
}

type FileInfo struct {
	URL        string `json:"url"`
	ExpiryTime string `json:"expiry_time"`
}

type ExternalInfo struct {
	URL string `json:"url"`
}

type RichText struct {
	Type     string `json:"type"`
	Text     *TextInfo `json:"text,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

type TextInfo struct {
	Content string `json:"content"`
	Link    *LinkInfo `json:"link,omitempty"`
}

type LinkInfo struct {
	URL string `json:"url"`
}

type SearchResponse struct {
	Object  string      `json:"object"`
	Results []NotionPage `json:"results"`
	HasMore bool        `json:"has_more"`
	NextCursor string   `json:"next_cursor"`
}

type BlocksResponse struct {
	Object     string       `json:"object"`
	Results    []NotionBlock `json:"results"`
	HasMore    bool         `json:"has_more"`
	NextCursor string       `json:"next_cursor"`
}

func makeRequest(method, endpoint string, body interface{}) ([]byte, error) {
	token := os.Getenv("NOTION_INTEGRATION_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("NOTION_INTEGRATION_TOKEN not set")
	}

	url := NotionAPI + endpoint
	fmt.Printf("\n[DEBUG] %s %s\n", method, url)

	var reqBody io.Reader
	if body != nil {
		jsonBody, _ := json.MarshalIndent(body, "", "  ")
		fmt.Printf("[DEBUG] Request body:\n%s\n", string(jsonBody))
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Notion-Version", NotionVersion)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[DEBUG] Status: %d\n", resp.Status)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		fmt.Printf("[DEBUG] Response:\n%s\n", string(respBody))
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func main() {
	fmt.Println("========== NOTION API EXPERIMENT ==========")
	fmt.Printf("Using API version: %s\n", NotionVersion)
	fmt.Printf("Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	// Test 1: Search for pages
	fmt.Println("\n[TEST 1] Search all pages...")
	searchResp, err := makeRequest("POST", "/search", map[string]interface{}{
		"filter": map[string]interface{}{
			"property": "object",
			"value":    "page",
		},
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	var search SearchResponse
	if err := json.Unmarshal(searchResp, &search); err != nil {
		fmt.Printf("Error parsing: %v\n", err)
		return
	}

	fmt.Printf("Found %d pages\n", len(search.Results))
	for i, page := range search.Results {
		fmt.Printf("  [%d] ID: %s\n", i, page.ID[:8])
		fmt.Printf("       Created: %s\n", page.CreatedTime)
		fmt.Printf("       Modified: %s\n", page.LastEditedTime)
		if props, ok := page.Properties["title"].(map[string]interface{}); ok {
			if titleArray, ok := props["title"].([]interface{}); ok && len(titleArray) > 0 {
				if titleObj, ok := titleArray[0].(map[string]interface{}); ok {
					if text, ok := titleObj["text"].(map[string]interface{}); ok {
						fmt.Printf("       Title: %s\n", text["content"])
					}
				}
			}
		}
	}

	// Test 2: Get page details
	if len(search.Results) > 0 {
		pageID := search.Results[0].ID
		fmt.Printf("\n[TEST 2] Get page details for %s...\n", pageID[:8])

		pageResp, err := makeRequest("GET", fmt.Sprintf("/pages/%s", pageID), nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			var page NotionPage
			if err := json.Unmarshal(pageResp, &page); err == nil {
				fmt.Printf("Page object: %s\n", page.Object)
				fmt.Printf("Properties count: %d\n", len(page.Properties))
				for key := range page.Properties {
					fmt.Printf("  - %s\n", key)
				}
			}
		}

		// Test 3: Get page blocks
		fmt.Printf("\n[TEST 3] Get page blocks for %s...\n", pageID[:8])
		blocksResp, err := makeRequest("GET", fmt.Sprintf("/blocks/%s/children", pageID), nil)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			var blocks BlocksResponse
			if err := json.Unmarshal(blocksResp, &blocks); err != nil {
				fmt.Printf("Error parsing blocks: %v\n", err)
			} else {
				fmt.Printf("Found %d blocks (has_more: %v)\n", len(blocks.Results), blocks.HasMore)
				for i, block := range blocks.Results {
					fmt.Printf("  [%d] Type: %s\n", i, block.Type)
					switch block.Type {
					case "paragraph":
						if block.Paragraph != nil && len(block.Paragraph.RichText) > 0 {
							fmt.Printf("       Content: %s\n", block.Paragraph.RichText[0].Text.Content)
						}
					case "heading_1", "heading_2", "heading_3":
						if block.Heading1 != nil && len(block.Heading1.RichText) > 0 {
							fmt.Printf("       Content: %s\n", block.Heading1.RichText[0].Text.Content)
						}
					}
				}
			}
		}
	}

	// Test 4: Query database
	databaseID := os.Getenv("NOTION_ID_DATABASE")
	if databaseID != "" {
		databaseID = cleanID(databaseID)
		fmt.Printf("\n[TEST 4] Query database %s...\n", databaseID[:8])

		dbResp, err := makeRequest("POST", fmt.Sprintf("/databases/%s/query", databaseID), map[string]interface{}{
			"page_size": 10,
		})
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			var result map[string]interface{}
			if err := json.Unmarshal(dbResp, &result); err == nil {
				fmt.Printf("Query result keys: %v\n", getMapKeys(result))
				if results, ok := result["results"].([]interface{}); ok {
					fmt.Printf("Found %d records\n", len(results))
				}
			}
		}
	}

	fmt.Println("\n========== EXPERIMENT COMPLETE ==========")
}

func cleanID(id string) string {
	// Remove hyphens and take first 32 chars
	clean := ""
	for _, ch := range id {
		if ch != '-' && ch != '?' {
			clean += string(ch)
			if len(clean) == 32 {
				break
			}
		}
	}
	return clean
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Movie represents a movie with its details
type Movie struct {
	Title         string
	Year          string
	URL           string
	CriticScore   string
	AudienceScore string
	Consensus     string
}

// SearchResult represents a movie search result
type SearchResult struct {
	Title string
	Year  string
	URL   string
}

// MovieList represents a serializable movie list
type MovieList struct {
	Movies []*Movie `json:"movies"`
}

// RottenTomatoesAPI handles interactions with Rotten Tomatoes
type RottenTomatoesAPI struct {
	baseURL string
	client  *http.Client
}

// NewRottenTomatoesAPI creates a new API client
func NewRottenTomatoesAPI() *RottenTomatoesAPI {
	return &RottenTomatoesAPI{
		baseURL: "https://www.rottentomatoes.com",
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// SearchMovies searches for movies by query term
// go function declaration syntax: the parenthesis after func is a function receiver!
func (api *RottenTomatoesAPI) SearchMovies(query string) ([]SearchResult, error) {
	searchURL := fmt.Sprintf("%s/search?search=%s", api.baseURL, url.QueryEscape(query))

	// Create request with headers to mimic a browser
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Make the request
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}

	/*
		The defer keyword schedules the Close() method to be called when the surrounding function returns, regardless of whether the function exits normally or due to an error. This ensures that resources associated with the response body are properly released.

		The Close() method for HTTP response bodies typically:

		Releases network resources associated with the connection
		Returns the connection to a connection pool for reuse (if applicable)
		Ensures any remaining unread data is drained
		Not closing response bodies is a common source of resource leaks in Go applications. Each unclosed response body can leave an open file descriptor or network connection, potentially leading to resource exhaustion over time. The defer resp.Body.Close() pattern is usually placed immediately after checking for request errors, ensuring cleanup happens regardless of how the function exits.
	*/
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search failed with status code: %d", resp.StatusCode)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	// Look for movie results in the search page
	// Method 1: Using newer RT layout
	doc.Find("search-page-media-row").Each(func(i int, item *goquery.Selection) {
		title := item.Find("[slot=title]").Text()
		title = strings.TrimSpace(title)

		if title == "" {
			return
		}

		url, _ := item.Find("a").Attr("href")
		if url != "" && !strings.HasPrefix(url, "http") {
			url = api.baseURL + url
		}

		year, _ := item.Attr("releaseyear")
		year = strings.TrimSpace(year)

		if len(year) != 4 {
			year = ""
		}

		// Try to get year
		if year == "" {
			year = item.Find("[slot=year]").Text()
			year = strings.TrimSpace(year)
		}

		// Fallback methods for year extraction
		if year == "" {
			// Try to extract from HTML
			html, _ := item.Html()
			re := regexp.MustCompile(`(\b(?:19|20)\d{2}\b)`)
			matches := re.FindStringSubmatch(html)
			if len(matches) > 1 {
				year = matches[1]
			}

			// Try to extract from URL
			if year == "" && strings.Contains(url, "/m/") {
				re := regexp.MustCompile(`/m/[^/]*_(\d{4})(?:/|$)`)
				matches := re.FindStringSubmatch(url)
				if len(matches) > 1 {
					year = matches[1]
				}
			}

			// Try to extract from title
			if year == "" {
				re := regexp.MustCompile(`\((\d{4})\)$`)
				matches := re.FindStringSubmatch(title)
				if len(matches) > 1 {
					year = matches[1]
				}
				// Try to extract from data attributes if available
				if year == "" {
					yearAttr, exists := item.Attr("data-year")
					if exists && yearAttr != "" {
						year = yearAttr
					}
				}
			}
		}

		if year == "" {
			year = "N/A"
		}

		results = append(results, SearchResult{
			Title: title,
			Year:  year,
			URL:   url,
		})
	})

	// ---- YEAR IN CASE OF OLDER RT LAYOUT ----
	// If no results found, try alternative selectors
	if len(results) == 0 {
		// Method 2: Using older RT layout
		doc.Find(".findify-components--cards__inner, .js-tile-link, .search__results .poster").Each(func(i int, item *goquery.Selection) {
			title := item.Find(".movieTitle").Text()
			title = strings.TrimSpace(title)

			if title == "" {
				return
			}

			url, _ := item.Find("a").Attr("href")
			if url != "" && !strings.HasPrefix(url, "http") {
				url = api.baseURL + url
			}

			// Try to get year
			year := item.Find(".movieYear").Text()
			year = strings.TrimSpace(year)

			// Fallback methods for year extraction (same as above)
			if year == "" {
				html, _ := item.Html()
				re := regexp.MustCompile(`(\b(?:19|20)\d{2}\b)`)
				matches := re.FindStringSubmatch(html)
				if len(matches) > 1 {
					year = matches[1]
				}

				if year == "" && strings.Contains(url, "/m/") {
					re := regexp.MustCompile(`/m/[^/]*_(\d{4})(?:/|$)`)
					matches := re.FindStringSubmatch(url)
					if len(matches) > 1 {
						year = matches[1]
					}
				}

				if year == "" {
					re := regexp.MustCompile(`\((\d{4})\)$`)
					matches := re.FindStringSubmatch(title)
					if len(matches) > 1 {
						year = matches[1]
						// Try to get year by pre-fetching the movie page
					}
				}
			}

			if year == "" {
				year = "N/A"
			}

			results = append(results, SearchResult{
				Title: title,
				Year:  year,
				URL:   url,
			})
		})
	}

	return results, nil
}

// getYearFromMoviePage extracts the year from a movie's page
// This is a last resort to get the year when it's not available in search results
func (api *RottenTomatoesAPI) getYearFromMoviePage(movieURL string) (string, error) {
	req, err := http.NewRequest("GET", movieURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Use a shorter timeout for this quick request
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("failed with status code: %d", resp.StatusCode)
	}

	// Parse HTML to find the year
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Get only part of the HTML content to save time
	htmlContent, err := doc.Html()
	if err != nil {
		return "", err
	}

	// Extract year using patterns
	yearPatterns := []string{
		`(?:dateCreated|release)["\']?\s*:\s*["\']?(?:[^"\']*?)(\d{4})`,
		`(?:release|released|year)[^<>\d]{1,20}((?:19|20)\d{2})`,
		`(?:movie|film)[^<>\d]{1,30}((?:19|20)\d{2})`,
	}

	for _, pattern := range yearPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(htmlContent); len(matches) > 1 {
			return matches[1], nil
		}
	}

	// Fallback to any year in the first part of HTML
	yearRe := regexp.MustCompile(`((?:19|20)\d{2})`)
	if matches := yearRe.FindStringSubmatch(htmlContent[0:5000]); len(matches) > 1 {
		return matches[1], nil
	}

	return "", nil
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", etc.
		cmd = "xdg-open"
	}
	args = append(args, url)

	return exec.Command(cmd, args...).Start()
}

// GetMovieDetails fetches detailed information for a specific movie
func (api *RottenTomatoesAPI) GetMovieDetails(movieURL string) (*Movie, error) {
	// Create request with headers to mimic a browser
	req, err := http.NewRequest("GET", movieURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Make the request
	resp, err := api.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to fetch movie details with status code: %d", resp.StatusCode)
	}

	// Parse the HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Get HTML content as string for regex operations
	htmlContent, err := doc.Html()
	if err != nil {
		return nil, err
	}

	movie := &Movie{
		Title:         "Unknown Title",
		Year:          "N/A",
		CriticScore:   "N/A",
		AudienceScore: "N/A",
		Consensus:     "No consensus yet.",
		URL:           movieURL,
	}

	// Extract movie title from meta tags (most reliable)
	titleRe := regexp.MustCompile(`<meta property="og:title" content="([^|]+)(?:\s*\|)?`)
	if matches := titleRe.FindStringSubmatch(htmlContent); len(matches) > 1 {
		movie.Title = strings.TrimSpace(matches[1])
	}

	// Fallback for title extraction
	if movie.Title == "Unknown Title" {
		// Try title tag
		titleTagRe := regexp.MustCompile(`<title>([^|]+)(?:\s*\|)?`)
		if matches := titleTagRe.FindStringSubmatch(htmlContent); len(matches) > 1 {
			movie.Title = strings.TrimSpace(matches[1])
		}
	}

	// Extract critic score from schema.org data (most reliable)
	criticScoreRe := regexp.MustCompile(`"aggregateRating".*?"ratingValue":"?(\d+)"?`)
	if matches := criticScoreRe.FindStringSubmatch(htmlContent); len(matches) > 1 {
		movie.CriticScore = matches[1]
	}

	// Fallback for critic score
	if movie.CriticScore == "N/A" {
		// Try other patterns
		patterns := []string{
			`(?:tomatometer|score)["\']?\s*:\s*["\']?(\d+)["\']?`,
			`(?:critic|tomatometer).*?(\d{1,3})%`,
			`data-qa="tomatometer"[^>]*>(\d+)%`,
		}

		for _, pattern := range patterns {
			re := regexp.MustCompile(pattern)
			if matches := re.FindStringSubmatch(htmlContent); len(matches) > 1 {
				movie.CriticScore = matches[1]
				break
			}
		}
	}

	// Extract audience score
	audiencePatterns := []string{
		`"audience[sS]core":\s*"?(\d+)"?`,
		`<score-board[^>]*audiencescore="(\d+)"`,
		`popcornmeter[^>]*>(\d+)%`,
		`popcornscore[^>]*>(\d+)%`,
		`audience-score[^>]*>(\d+)%`,
		`audiencescore["\']?\s*:\s*["\']?(\d+)["\']?`,
		`audience[^%<>]*?(\d{1,3})%`,
		`data-qa="audience-score"[^>]*>(\d+)%`,
		`"audienceScore":"(\d+)"`,
		`<span class="audience-score">(\d+)%</span>`,
	}

	for _, pattern := range audiencePatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(htmlContent); len(matches) > 1 {
			movie.AudienceScore = matches[1]
			break
		}
	}

	// Extract year
	yearPatterns := []string{
		`(?:dateCreated|release)["\']?\s*:\s*["\']?(?:[^"\']*?)(\d{4})`,
		`(?:release|released|year)[^<>\d]{1,20}((?:19|20)\d{2})`,
		`(?:movie|film)[^<>\d]{1,30}((?:19|20)\d{2})`,
	}

	for _, pattern := range yearPatterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(htmlContent); len(matches) > 1 {
			movie.Year = matches[1]
			break
		}
	}

	// Fallback for year
	if movie.Year == "N/A" {
		yearRe := regexp.MustCompile(`((?:19|20)\d{2})`)
		if matches := yearRe.FindStringSubmatch(htmlContent[0:5000]); len(matches) > 1 {
			movie.Year = matches[1]
		}
	}

	// Extract critics consensus using BeautifulSoup-like approach with goquery
	consensusFound := false

	// Try various selectors
	consensusSelectors := []string{
		"[data-qa=\"critics-consensus\"]",
		".criticsconsensus",
		".critic_consensus",
		".what-to-know__consensus",
		".consensus",
		"p.consensus",
	}

	for _, selector := range consensusSelectors {
		if consensus := doc.Find(selector).First(); consensus.Length() > 0 {
			text := strings.TrimSpace(consensus.Text())
			if len(text) > 20 {
				movie.Consensus = text
				consensusFound = true
				break
			}
		}
	}

	// Fallback methods for consensus
	if !consensusFound {
		// Try structured data
		consensusRe := regexp.MustCompile(`"reviewBody":\s*"([^"]{20,})"`)
		if matches := consensusRe.FindStringSubmatch(htmlContent); len(matches) > 1 {
			movie.Consensus = strings.TrimSpace(matches[1])
			consensusFound = true
		}
	}

	// Try looking for paragraphs that might contain consensus
	if !consensusFound || strings.Contains(movie.Consensus, "class=") {
		// Check all paragraphs
		doc.Find("p").Each(func(i int, p *goquery.Selection) {
			text := strings.TrimSpace(p.Text())
			// Look for paragraphs with typical consensus keywords
			if len(text) > 30 && len(text) < 500 &&
				(strings.Contains(strings.ToLower(text), "director") ||
					strings.Contains(strings.ToLower(text), "cinematic") ||
					strings.Contains(strings.ToLower(text), "visually") ||
					strings.Contains(strings.ToLower(text), "narrative") ||
					strings.Contains(strings.ToLower(text), "performance") ||
					strings.Contains(strings.ToLower(text), "ambitious")) {
				// Check that it's not a plot description
				if !strings.Contains(strings.ToLower(text), "subconscious") &&
					!strings.Contains(strings.ToLower(text), "mission") {
					movie.Consensus = text
					consensusFound = true
					return
				}
			}
		})
	}

	// Clean up consensus text
	if consensusFound {
		// Remove "Critics Consensus:" prefix if present
		consensusPrefixRe := regexp.MustCompile(`(?i)^Critics\s+Consensus:?\s*`)
		movie.Consensus = consensusPrefixRe.ReplaceAllString(movie.Consensus, "")

		// Clean HTML if needed
		if strings.Contains(movie.Consensus, "class=") || strings.Contains(movie.Consensus, "<") || strings.Contains(movie.Consensus, ">") {
			htmlTagsRe := regexp.MustCompile(`<[^>]*>`)
			movie.Consensus = htmlTagsRe.ReplaceAllString(movie.Consensus, " ")

			// Normalize whitespace
			whitespaceRe := regexp.MustCompile(`\s+`)
			movie.Consensus = strings.TrimSpace(whitespaceRe.ReplaceAllString(movie.Consensus, " "))
		}

		// Replace HTML entities
		movie.Consensus = strings.ReplaceAll(movie.Consensus, "&nbsp;", " ")
		movie.Consensus = strings.ReplaceAll(movie.Consensus, "&quot;", "\"")
		movie.Consensus = strings.ReplaceAll(movie.Consensus, "&#39;", "'")
		movie.Consensus = strings.ReplaceAll(movie.Consensus, "&amp;", "&")
	}

	return movie, nil
}

// SaveMovieLists saves all movie lists to a JSON file
func SaveMovieLists(lists map[string][]*Movie) error {
	// Create .config directory if it doesn't exist
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(homeDir, ".config", "rotten-cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	listsFilePath := filepath.Join(configDir, "lists.json")

	// Convert map to a serializable format
	serializedLists := make(map[string]MovieList)
	for name, movies := range lists {
		serializedLists[name] = MovieList{Movies: movies}
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(serializedLists, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(listsFilePath, data, 0644)
}

// LoadMovieLists loads all movie lists from a JSON file
func LoadMovieLists() (map[string][]*Movie, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	listsFilePath := filepath.Join(homeDir, ".config", "rotten-cli", "lists.json")

	// Check if file exists
	if _, err := os.Stat(listsFilePath); os.IsNotExist(err) {
		// Return empty lists if file doesn't exist yet
		return make(map[string][]*Movie), nil
	}

	// Read file
	data, err := os.ReadFile(listsFilePath)
	if err != nil {
		return nil, err
	}

	// Unmarshal JSON
	var serializedLists map[string]MovieList
	if err := json.Unmarshal(data, &serializedLists); err != nil {
		return nil, err
	}

	// Convert to expected format
	lists := make(map[string][]*Movie)
	for name, list := range serializedLists {
		lists[name] = list.Movies
	}

	return lists, nil
}

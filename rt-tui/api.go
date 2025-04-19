package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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
			Timeout: 10 * time.Second,
		},
	}
}

// SearchMovies searches for movies by query term
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
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search failed with status code: %d", resp.StatusCode)
	}

	// Save a portion of the HTML for debugging - TO REMOVE IN PRODUCTION
	htmlContent, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	html, _ := htmlContent.Html()
	// Save the first 50,000 characters of HTML for debugging
	err = os.WriteFile("debug_search.txt", []byte(html[:50000]), 0644)
	if err != nil {
		fmt.Println("Error saving debug file:", err)
	}

	// Re-parse the HTML from the saved string (because we've already read the body)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	var results []SearchResult

	// Look for movie results in the search page
	// Method 1: Using newer RT layout
	doc.Find("search-page-media-row").Each(func(i int, item *goquery.Selection) {
		// Extract and debug the whole row HTML
		rowHTML, _ := item.Html()
		fmt.Printf("Found search-page-media-row: %s\n", rowHTML[:100]) // Print first 100 chars

		title := item.Find("[slot=title]").Text()
		title = strings.TrimSpace(title)

		if title == "" {
			return
		}

		url, _ := item.Find("a").Attr("href")
		if url != "" && !strings.HasPrefix(url, "http") {
			url = api.baseURL + url
		}

		 // Debug found title and URL
		fmt.Printf("Found title: %s, URL: %s\n", title, url)

		// Try direct approach for year
		year := ""

		// Check for year in dedicated slot
		yearElem := item.Find("[slot=year]")
		if yearElem.Length() > 0 {
			year = strings.TrimSpace(yearElem.Text())
			fmt.Printf("Found year in slot=year: %s\n", year)
		}

		// Try to get year from release date text
		if year == "" {
			releaseDate := item.Find("[slot=releaseDate]")
			if releaseDate.Length() > 0 {
				releaseDateText := strings.TrimSpace(releaseDate.Text())
				fmt.Printf("Found release date text: %s\n", releaseDateText)

				// Extract year from release date
				re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
				if matches := re.FindStringSubmatch(releaseDateText); len(matches) > 1 {
					year = matches[1]
					fmt.Printf("Extracted year from release date: %s\n", year)
				}
			}
		}

		// Check for year in metadata slot
		if year == "" {
			metaElem := item.Find("[slot=metadata]")
			if metaElem.Length() > 0 {
				metaText := strings.TrimSpace(metaElem.Text())
				fmt.Printf("Found metadata text: %s\n", metaText)

				// Extract year pattern
				re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
				if matches := re.FindStringSubmatch(metaText); len(matches) > 1 {
					year = matches[1]
					fmt.Printf("Extracted year from metadata: %s\n", year)
				}
			}
		}

		// Try to find year in any other available slots
		if year == "" {
			item.Find("[slot]").Each(func(i int, slotElem *goquery.Selection) {
				slotName, exists := slotElem.Attr("slot")
				if exists {
					slotText := strings.TrimSpace(slotElem.Text())
					fmt.Printf("Found slot %s with text: %s\n", slotName, slotText)

					// Try to extract year from this slot
					re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
					if matches := re.FindStringSubmatch(slotText); len(matches) > 1 {
						year = matches[1]
						fmt.Printf("Extracted year from slot %s: %s\n", slotName, year)
					}
				}
			})
		}

		// Try all remaining fallback methods
		if year == "" {
			// Try to extract from HTML
			html, _ := item.Html()
			re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
			matches := re.FindStringSubmatch(html)
			if len(matches) > 1 {
				year = matches[1]
				fmt.Printf("Extracted year from HTML: %s\n", year)
			}

			// Try to extract from URL
			if year == "" && strings.Contains(url, "/m/") {
				re := regexp.MustCompile(`/m/[^/]*_(\d{4})(?:/|$)`)
				matches := re.FindStringSubmatch(url)
				if len(matches) > 1 {
					year = matches[1]
					fmt.Printf("Extracted year from URL: %s\n", year)
				}
			}

			// Try to extract from title
			if year == "" {
				re := regexp.MustCompile(`\((\d{4})\)$`)
				matches := re.FindStringSubmatch(title)
				if len(matches) > 1 {
					year = matches[1]
					fmt.Printf("Extracted year from title: %s\n", year)
				}
			}

			// Try to extract from data attributes
			if year == "" {
				yearAttr, exists := item.Attr("data-year")
				if exists && yearAttr != "" {
					year = yearAttr
					fmt.Printf("Extracted year from data-year attribute: %s\n", year)
				}

				// Look for release date attribute
				dateAttr, exists := item.Attr("data-release-date")
				if exists && dateAttr != "" {
					re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
					matches := re.FindStringSubmatch(dateAttr)
					if len(matches) > 1 {
						year = matches[1]
						fmt.Printf("Extracted year from data-release-date attribute: %s\n", year)
					}
				}
			}
		}

		// If still no year, use placeholder
		if year == "" {
			year = "---"
		}

		results = append(results, SearchResult{
			Title: title,
			Year:  year,
			URL:   url,
		})
	})

	// If no results found using the newer layout, try the alternative layout
	if len(results) == 0 {
		// Method 2: Try alternative selectors for older RT layout
		alternativeSelectors := []string{
			".findify-components--cards__inner",
			".js-tile-link",
			".search__results .poster",
			".search-page-result",
			"[data-qa='search-result']",
		}

		for _, selector := range alternativeSelectors {
			doc.Find(selector).Each(func(i int, item *goquery.Selection) {
				// Debug the found element
				elemHTML, _ := item.Html()
				fmt.Printf("Found alternative search result: %s\n", elemHTML[:100]) // Print first 100 chars

				// Try different title selectors
				titleSelectors := []string{
					".movieTitle",
					"p.title",
					"h3.title",
					"[data-qa='search-result-title']",
					"a.title",
					".movie_title",
				}

				var title string
				for _, titleSelector := range titleSelectors {
					titleElem := item.Find(titleSelector)
					if titleElem.Length() > 0 {
						title = strings.TrimSpace(titleElem.Text())
						fmt.Printf("Found title using selector %s: %s\n", titleSelector, title)
						break
					}
				}

				// Skip if title is empty
				if title == "" {
					// Last resort: try any h3 element
					title = strings.TrimSpace(item.Find("h3").Text())
					if title == "" {
						return
					}
				}

				// Get URL
				url, _ := item.Find("a").Attr("href")
				if url != "" && !strings.HasPrefix(url, "http") {
					url = api.baseURL + url
				}

				// Try various methods to get year
				year := ""

				// Try year-specific classes
				yearSelectors := []string{
					".movieYear",
					".year",
					".release-year",
					"[data-qa='release-year']",
				}

				for _, yearSelector := range yearSelectors {
					yearElem := item.Find(yearSelector)
					if yearElem.Length() > 0 {
						year = strings.TrimSpace(yearElem.Text())
						fmt.Printf("Found year using selector %s: %s\n", yearSelector, year)

						// Extract just the 4-digit year if there's additional text
						re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
						if matches := re.FindStringSubmatch(year); len(matches) > 1 {
							year = matches[1]
						}

						if year != "" {
							break
						}
					}
				}

				// Fallback methods for year extraction
				if year == "" {
					// Look for subtitle or release info that might contain year
					subtitleSelectors := []string{
						".movieSubtitle",
						".subtle",
						".release-date",
						".meta-data",
					}

					for _, subtitleSelector := range subtitleSelectors {
						subtitleElem := item.Find(subtitleSelector)
						if subtitleElem.Length() > 0 {
							subtitleText := strings.TrimSpace(subtitleElem.Text())
							fmt.Printf("Found subtitle using selector %s: %s\n", subtitleSelector, subtitleText)

							re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
							if matches := re.FindStringSubmatch(subtitleText); len(matches) > 1 {
								year = matches[1]
								fmt.Printf("Extracted year from subtitle: %s\n", year)
								break
							}
						}
					}

					// Try to extract from HTML
					if year == "" {
						html, _ := item.Html()
						re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
						matches := re.FindStringSubmatch(html)
						if len(matches) > 1 {
							year = matches[1]
							fmt.Printf("Extracted year from HTML: %s\n", year)
						}
					}

					// Try to extract from URL
					if year == "" && strings.Contains(url, "/m/") {
						re := regexp.MustCompile(`/m/[^/]*_(\d{4})(?:/|$)`)
						matches := re.FindStringSubmatch(url)
						if len(matches) > 1 {
							year = matches[1]
							fmt.Printf("Extracted year from URL: %s\n", year)
						}
					}

					// Try to extract from title
					if year == "" {
						re := regexp.MustCompile(`\((\d{4})\)$`)
						matches := re.FindStringSubmatch(title)
						if len(matches) > 1 {
							year = matches[1]
							fmt.Printf("Extracted year from title: %s\n", year)
						}
					}
				}

				if year == "" {
					year = "---"
				}

				results = append(results, SearchResult{
					Title: title,
					Year:  year,
					URL:   url,
				})
			})

			// If we found results with this selector, stop trying others
			if len(results) > 0 {
				break
			}
		}
	}

	// If still no results, try an additional approach for very different layouts
	if len(results) == 0 {
		// Look for any container with links that might be search results
		doc.Find("a").Each(func(i int, link *goquery.Selection) {
			href, exists := link.Attr("href")
			if !exists || !strings.Contains(href, "/m/") {
				return
			}

			// This looks like a movie link
			if !strings.HasPrefix(href, "http") {
				href = api.baseURL + href
			}

			// Try to get title from link text or contained elements
			title := strings.TrimSpace(link.Text())

			// If the link itself doesn't have text, look for child elements
			if title == "" {
				title = strings.TrimSpace(link.Find("h2, h3, p.title, .title").Text())
			}

			// Skip if no title found
			if title == "" {
				return
			}

			// Extract year using our patterns
			year := ""

			// Try to extract from URL
			if strings.Contains(href, "/m/") {
				re := regexp.MustCompile(`/m/[^/]*_(\d{4})(?:/|$)`)
				matches := re.FindStringSubmatch(href)
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
			}

			// Try to extract from nearby text
			if year == "" {
				// Look at parent and siblings for year
				parentHTML, _ := link.Parent().Html()
				re := regexp.MustCompile(`\b(19\d{2}|20\d{2})\b`)
				matches := re.FindStringSubmatch(parentHTML)
				if len(matches) > 1 {
					year = matches[1]
				}
			}

			if year == "" {
				year = "---"
			}

			results = append(results, SearchResult{
				Title: title,
				Year:  year,
				URL:   href,
			})
		})
	}

	return results, nil
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

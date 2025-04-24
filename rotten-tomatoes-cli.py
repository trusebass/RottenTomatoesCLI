import requests
from bs4 import BeautifulSoup
import argparse
import sys
import re
from urllib.parse import quote
import json

class RottenTomatoesAPI:
    def __init__(self):
        self.base_url = "https://www.rottentomatoes.com"
        self.headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
        }
    
    def search_movie(self, query):
        """Search for a movie and return a list of search results."""
        search_url = f"{self.base_url}/search?search={quote(query)}"
        print(f"Searching URL: {search_url}")
        response = requests.get(search_url, headers=self.headers)
        
        if response.status_code != 200:
            print(f"Search failed with status code: {response.status_code}")
            return []
        
        soup = BeautifulSoup(response.text, 'html.parser')
        search_results = []
        
        # Look for movie results in the search page
        movie_items = soup.select('search-page-media-row')
        print(f"Found {len(movie_items)} items with selector 'search-page-media-row'")
        
        if not movie_items:
            # Alternate method to find movies
            movie_items = soup.select('.findify-components--cards__inner')
            print(f"Found {len(movie_items)} items with alternate selector '.findify-components--cards__inner'")
            
            # Try another selector for movie items
            if not movie_items:
                movie_items = soup.select('.js-tile-link') or soup.select('.search__results .poster')
                print(f"Found {len(movie_items)} items with additional selectors")
        
        # Save the search results HTML for debugging
        with open("debug_search.txt", "w", encoding="utf-8") as f:
            f.write(str(soup)[:10000])
        
        for item in movie_items:
            try:
                # Extract title
                title_elem = item.select_one('[slot="title"]') or item.select_one('.movieTitle')
                if not title_elem:
                    continue
                
                title = title_elem.text.strip()
                
                # Get the movie URL
                url_elem = item.select_one('a') or title_elem.parent
                if not url_elem:
                    continue
                
                url = url_elem.get('href', '')
                if url and not url.startswith('http'):
                    url = self.base_url + url
                
                # Try multiple approaches to extract the year
                year = "N/A"
                
                # Method 1: Using dedicated year element
                year_elem = item.select_one('[slot="year"]') or item.select_one('.movieYear')
                if year_elem:
                    year = year_elem.text.strip()
                
                # Method 2: Look for year in the item HTML
                if year == "N/A":
                    # Look for a year pattern in the item's HTML
                    item_html = str(item)
                    year_match = re.search(r'(\b(?:19|20)\d{2}\b)', item_html)
                    if year_match:
                        year = year_match.group(1)
                
                # Method 3: Extract year from URL if it contains year info
                if year == "N/A" and '/m/' in url:
                    # URLs sometimes contain the year, like /m/movie_title_2010
                    url_year_match = re.search(r'/m/[^/]*_(\d{4})(?:/|$)', url)
                    if url_year_match:
                        year = url_year_match.group(1)
                
                # Method 4: Try to extract from movie title if it ends with a year
                if year == "N/A":
                    title_year_match = re.search(r'\((\d{4})\)$', title)
                    if title_year_match:
                        year = title_year_match.group(1)
                
                search_results.append({
                    'title': title,
                    'year': year,
                    'url': url
                })
                print(f"Added result: {title} ({year}) - {url}")
            except Exception as e:
                print(f"Error parsing search result: {e}")
        
        return search_results
    
    def get_movie_ratings(self, movie_url):
        """Get the ratings for a specific movie."""
        print(f"Fetching data from: {movie_url}")
        response = requests.get(movie_url, headers=self.headers)
        
        if response.status_code != 200:
            print(f"Failed to fetch movie data: {response.status_code}")
            return None
        
        html_content = response.text
        
        # Save HTML to debug file
        with open("debug_html.txt", "w", encoding="utf-8") as f:
            f.write(html_content[:30000])  # Save more of the HTML to capture consensus text
        
        # Parse the HTML with BeautifulSoup for more reliable extraction
        soup = BeautifulSoup(html_content, 'html.parser')
        
        # Initialize default values
        title = "Unknown Title"
        critic_score = "N/A"
        audience_score = "N/A"
        consensus_text = "No consensus yet."
        year = "N/A"
        
        # Debug data collection
        debug_info = {}
        
        # Extract movie title from meta tags (most reliable)
        title_pattern = r'<meta property="og:title" content="([^|]+)(?:\s*\|)?'
        title_match = re.search(title_pattern, html_content)
        if title_match:
            title = title_match.group(1).strip()
            debug_info['title_from_meta'] = title
        
        # Try alternate title methods if needed
        if title == "Unknown Title":
            schema_pattern = r'"@type":"Movie".*?"name":"([^"]+)"'
            schema_match = re.search(schema_pattern, html_content, re.DOTALL)
            if schema_match:
                title = schema_match.group(1)
                debug_info['title_from_schema'] = title
            else:
                # Look for title in HTML title tag
                title_tag_match = re.search(r'<title>([^|]+)(?:\s*\|)?', html_content)
                if title_tag_match:
                    title = title_tag_match.group(1).strip()
                    debug_info['title_from_title_tag'] = title
        
        # Extract critic score (Tomatometer) 
        # Method 1: Look for aggregateRating in schema.org markup (most reliable)
        critic_pattern1 = r'"aggregateRating".*?"ratingValue":"?(\d+)"?'
        critic_match = re.search(critic_pattern1, html_content, re.DOTALL)
        if critic_match:
            critic_score = critic_match.group(1)
            debug_info['critic_score_from_schema'] = critic_score
        
        # Method 2: Look for tomatometer score in various JavaScript locations
        if critic_score == "N/A":
            critic_patterns = [
                r'(?:tomatometer|score)["\']?\s*:\s*["\']?(\d+)["\']?',
                r'(?:critic|tomatometer).*?(\d{1,3})%',
                r'data-qa="tomatometer"[^>]*>(\d+)%'
            ]
            
            for pattern in critic_patterns:
                match = re.search(pattern, html_content, re.IGNORECASE)
                if match:
                    critic_score = match.group(1)
                    debug_info['critic_score_pattern'] = pattern
                    break
        
        # Extract audience score - IMPROVED METHODS
        # Method 1: First look for audienceScore in JSON-LD data
        audience_json_pattern = r'"audience[sS]core":\s*"?(\d+)"?'
        audience_match = re.search(audience_json_pattern, html_content)
        if audience_match:
            audience_score = audience_match.group(1)
            debug_info['audience_score_from_json'] = True
        
        # Method 2: Look for score-board element with audiencescore attribute
        if audience_score == "N/A":
            scoreboard_pattern = r'<score-board[^>]*audiencescore="(\d+)"'
            scoreboard_match = re.search(scoreboard_pattern, html_content)
            if scoreboard_match:
                audience_score = scoreboard_match.group(1)
                debug_info['audience_score_from_scoreboard'] = True
        
        # Method 3: Try to find audience score in "popcornmeter" elements
        if audience_score == "N/A":
            popcorn_patterns = [
                r'popcornmeter[^>]*>(\d+)%',
                r'popcornscore[^>]*>(\d+)%',
                r'audience-score[^>]*>(\d+)%'
            ]
            
            for pattern in popcorn_patterns:
                match = re.search(pattern, html_content, re.IGNORECASE)
                if match:
                    audience_score = match.group(1)
                    debug_info['audience_score_from_popcorn'] = pattern
                    break
        
        # Method 4: Use additional audience patterns as fallback
        if audience_score == "N/A":
            audience_patterns = [
                r'audiencescore["\']?\s*:\s*["\']?(\d+)["\']?',
                r'audience[^%<>]*?(\d{1,3})%',
                r'data-qa="audience-score"[^>]*>(\d+)%',
                # More specific RT patterns
                r'"audienceScore":"(\d+)"',
                r'<span class="audience-score">(\d+)%</span>'
            ]
            
            for pattern in audience_patterns:
                match = re.search(pattern, html_content, re.IGNORECASE)
                if match:
                    audience_score = match.group(1)
                    debug_info['audience_score_pattern'] = pattern
                    break
        
        # Extract year using multiple patterns
        year_patterns = [
            r'(?:dateCreated|release)["\']?\s*:\s*["\']?(?:[^"\']*?)(\d{4})', 
            r'(?:release|released|year)[^<>\d]{1,20}((?:19|20)\d{2})',
            r'(?:movie|film)[^<>\d]{1,30}((?:19|20)\d{2})'
        ]
        
        for pattern in year_patterns:
            match = re.search(pattern, html_content, re.IGNORECASE)
            if match:
                year = match.group(1)
                debug_info['year_pattern'] = pattern
                break
        
        # Fallback for year: look for any 4-digit year in the first part of HTML
        if year == "N/A":
            year_match = re.search(r'((?:19|20)\d{2})', html_content[:5000])
            if year_match:
                year = year_match.group(1)
                debug_info['year_from_fallback'] = True
        
        # FIXED CRITICS CONSENSUS EXTRACTION - uses BeautifulSoup for more reliable extraction
        # Method 1: Try to extract consensus using BeautifulSoup selectors
        consensus_found = False
        
        # First try with BeautifulSoup - more reliable than regex for HTML parsing
        consensus_selectors = [
            '[data-qa="critics-consensus"]',
            '.criticsconsensus',
            '.critic_consensus',
            '.what-to-know__consensus',
            '.consensus',
            'p.consensus'
        ]
        
        for selector in consensus_selectors:
            consensus_elem = soup.select_one(selector)
            if consensus_elem:
                # Get the text content, not the HTML
                potential_consensus = consensus_elem.get_text(strip=True)
                if len(potential_consensus) > 20:
                    consensus_text = potential_consensus
                    consensus_found = True
                    debug_info['consensus_from_selector'] = selector
                    break
        
        # Method 2: If BeautifulSoup fails, try to use regex patterns with better boundaries
        if not consensus_found:
            # Try to extract from structured data
            schema_consensus_pattern = r'"reviewBody":\s*"([^"]{20,})"'
            schema_match = re.search(schema_consensus_pattern, html_content)
            if schema_match:
                consensus_text = schema_match.group(1).strip()
                consensus_found = True
                debug_info['consensus_from_schema_data'] = True
        
        # Method 3: Look for elements with explicit consensus markers using better regex
        if not consensus_found:
            consensus_markers = [
                r'<p\s+class="[^"]*critic_consensus[^"]*"[^>]*>(.*?)</p>',
                r'<span\s+data-qa="critics-consensus"[^>]*>(.*?)</span>',
                r'<div\s+class="[^"]*consensus[^"]*"[^>]*>(.*?)</div>',
                # Fixed pattern to avoid capturing HTML attributes
                r'data-qa="critics-consensus"[^>]*>(.*?)</span>',
                r'<p\s+class="consensus"[^>]*>(.*?)</p>'
            ]
            
            for pattern in consensus_markers:
                match = re.search(pattern, html_content, re.IGNORECASE | re.DOTALL)
                if match:
                    potential_consensus = match.group(1).strip()
                    # Clean and verify - remove any remaining HTML tags
                    potential_consensus = re.sub(r'<[^>]+>', ' ', potential_consensus)
                    potential_consensus = re.sub(r'\s+', ' ', potential_consensus).strip()
                    
                    if len(potential_consensus) > 20 and 'class=' not in potential_consensus:
                        consensus_text = potential_consensus
                        consensus_found = True
                        debug_info['consensus_from_marker'] = pattern
                        break
        
        # Method 4: Search for explicit "Critics Consensus:" text and capture what follows
        if not consensus_found:
            explicit_patterns = [
                r'Critics\s+Consensus:\s*(.*?)(?:</p>|</div>|<br|$)',
                r'Consensus:\s*(.*?)(?:</p>|</div>|<br|$)'
            ]
            
            for pattern in explicit_patterns:
                match = re.search(pattern, html_content, re.IGNORECASE | re.DOTALL)
                if match:
                    consensus_text = match.group(1).strip()
                    # Clean up the text
                    consensus_text = re.sub(r'<[^>]+>', ' ', consensus_text)
                    consensus_text = re.sub(r'\s+', ' ', consensus_text).strip()
                    
                    if len(consensus_text) > 20 and 'class=' not in consensus_text:
                        consensus_found = True
                        debug_info['consensus_from_explicit'] = pattern
                        break
        
        # Method 5: Try to find paragraphs that look like consensus text
        if not consensus_found or 'class=' in consensus_text:
            # Final attempt - check the document for any paragraph that might be consensus
            paragraphs = soup.find_all('p')
            for p in paragraphs:
                text = p.get_text(strip=True)
                # Look for paragraphs with keywords typical of consensus statements
                if (len(text) > 30 and len(text) < 500 and 
                    ('director' in text.lower() or 'cinematic' in text.lower() or 
                     'visually' in text.lower() or 'narrative' in text.lower() or
                     'performance' in text.lower() or 'ambitious' in text.lower())):
                    # Check if this is not a plot description
                    if 'subconscious' not in text.lower() and 'mission' not in text.lower():
                        consensus_text = text
                        consensus_found = True
                        debug_info['consensus_from_paragraph'] = True
                        break
        
        # Clean up the consensus text to make it presentable
        if consensus_found:
            # Remove "Critics Consensus:" prefix if present
            consensus_text = re.sub(r'^Critics\s+Consensus:?\s*', '', consensus_text, flags=re.IGNORECASE)
            # Make sure we're not capturing HTML
            if 'class=' in consensus_text or '<' in consensus_text or '>' in consensus_text:
                # If we captured HTML, try a more aggressive cleanup
                consensus_text = re.sub(r'<[^>]*>', ' ', consensus_text)
                consensus_text = re.sub(r'\s+', ' ', consensus_text).strip()
            # Replace HTML entities with proper characters
            consensus_text = consensus_text.replace('&nbsp;', ' ').replace('&quot;', '"').replace('&#39;', "'").replace('&amp;', '&')
        
        # Print results to console for debugging
        print(f"Title: {title}")
        print(f"Year: {year}")
        print(f"Critic Score: {critic_score}")
        print(f"Audience Score: {audience_score}")
        print(f"Consensus: {consensus_text[:100]}...")
        
        # Save debug info
        with open("debug_movie_info.txt", "w", encoding="utf-8") as f:
            f.write(f"Title: {title}\n")
            f.write(f"Year: {year}\n")
            f.write(f"Critic Score: {critic_score}\n")
            f.write(f"Audience Score: {audience_score}\n")
            f.write(f"Consensus: {consensus_text}\n")
            f.write("\nDebug Info:\n")
            for key, value in debug_info.items():
                f.write(f"{key}: {value}\n")
        
        return {
            'title': title,
            'year': year,
            'critic_score': critic_score,
            'audience_score': audience_score,
            'consensus': consensus_text,
            'url': movie_url
        }

def display_movie_info(movie):
    """Display movie information in a user-friendly format."""
    print("\n" + "=" * 60)
    print(f"Title: {movie['title']} ({movie['year']})")
    print("-" * 60)
    print(f"Tomatometer (Critics): {movie['critic_score']}%")
    print(f"Popcornometer (Audience): {movie['audience_score']}%")
    print("-" * 60)
    print("Critics Consensus:")
    print(movie['consensus'])
    print("-" * 60)
    print(f"More info: {movie['url']}")
    print("=" * 60 + "\n")

def main():
    parser = argparse.ArgumentParser(description='Search for movie ratings on Rotten Tomatoes')
    parser.add_argument('query', nargs='*', help='Movie title to search for')
    args = parser.parse_args()
    
    rt_api = RottenTomatoesAPI()
    
    # If no query provided, prompt the user
    if not args.query:
        search_query = input("Enter movie title to search: ")
    else:
        search_query = ' '.join(args.query)
    
    print(f"\nSearching for '{search_query}'...")
    search_results = rt_api.search_movie(search_query)
    
    if not search_results:
        print("No results found. Try a different search term.")
        return
    
    # Display search results
    print(f"\nFound {len(search_results)} results:\n")
    for i, movie in enumerate(search_results, 1):
        print(f"{i}. {movie['title']} ({movie['year']})")
    
    # Let user select a movie
    selection = -1
    while not (0 <= selection < len(search_results)):
        try:
            selection = int(input(f"\nSelect a movie (1-{len(search_results)}) or 0 to quit: ")) - 1
            if selection == -1:
                print("Exiting...")
                return
        except ValueError:
            print("Please enter a valid number.")
    
    # Get and display ratings for the selected movie
    selected_movie = search_results[selection]
    print(f"\nFetching ratings for '{selected_movie['title']}'...")
    
    movie_details = rt_api.get_movie_ratings(selected_movie['url'])
    if movie_details:
        display_movie_info(movie_details)
    else:
        print("Failed to retrieve movie ratings.")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        print("\nExiting...")
        sys.exit(0)

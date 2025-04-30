package main

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/paginator"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styling constants
var (
	// Colors
	primaryColor   = lipgloss.Color("#E50914") // Netflix red, close to RT's color
	secondaryColor = lipgloss.Color("#F5F5F1") // Light cream color
	accentColor    = lipgloss.Color("#564D4D") // Dark gray
	//bgColor        = lipgloss.Color("#171717") // Near black

	// Text styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1) // Removed alignment and width properties

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			Align(lipgloss.Center)

	normalTextStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Align(lipgloss.Center)

	highlightedTextStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	scoreStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true).
			Align(lipgloss.Center)

	// Component styles
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1).
			Width(40)

	movieListStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1).
			Width(60)

	selectedMovieStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1).
				Width(60)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
)

// Application states
const (
	stateSearchInput = iota
	stateSearching
	stateMovieList
	stateLoadingMovie
	stateMovieDetails
	stateError
	stateListManagement // Now shows all lists with Add List option
	stateCreateList
	stateViewListDetails
	stateListSelection
)

// Model represents the application state
type Model struct {
	state        int
	api          *RottenTomatoesAPI
	textInput    textinput.Model
	spinner      spinner.Model
	results      []SearchResult
	selectedIdx  int
	movie        *Movie
	error        error
	viewport     viewport.Model
	width        int
	height       int
	movieLists   map[string][]*Movie
	selectedList string
	paginator    paginator.Model // Add paginator for main pages
	mainPage     int            // Track which main page we're on (0=search, 1=lists)
}

// NewModel creates a new application model
func NewModel() Model {
	// Set up text input for search
	ti := textinput.New()
	ti.Placeholder = "Enter a movie title to search..."
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	// Create explicit key mappings for Option+Backspace (Alt+Backspace)
	ti.KeyMap.DeleteWordBackward = key.NewBinding(
		key.WithKeys("option+backspace"),
	)
	ti.KeyMap.DeleteWordBackward.SetEnabled(true)

	// Set up spinner for loading states
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(primaryColor)

	// Set up viewport for scrollable content
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		BorderForeground(accentColor).
		Align(lipgloss.Center) // Center content within viewport
        
	// Set up paginator for main pages (Search and Lists)
	pg := paginator.New()
	pg.Type = paginator.Dots
	pg.ActiveDot = lipgloss.NewStyle().Foreground(primaryColor).Render("● ")  // Larger dot with spacing
	pg.InactiveDot = lipgloss.NewStyle().Foreground(accentColor).Render("○ ") // Larger inactive dot with spacing
	pg.SetTotalPages(2)
	pg.PerPage = 1
	pg.Page = 0 // Start on search page

	return Model{
		state:       stateSearchInput,
		api:         NewRottenTomatoesAPI(),
		textInput:   ti,
		spinner:     sp,
		results:     []SearchResult{},
		selectedIdx: 0,
		viewport:    vp,
		width:       80,
		height:      24,
		movieLists:  make(map[string][]*Movie),
		paginator:   pg,
		mainPage:    0, // Start on search page (0=search, 1=lists)
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		m.spinner.Tick,
		func() tea.Msg {
			// Load saved movie lists
			lists, err := LoadMovieLists()
			if err != nil {
				return errorMsg{fmt.Errorf("failed to load movie lists: %v", err)}
			}
			return listsLoadedMsg{lists: lists}
		},
	)
}

// Update handles messages and user input
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key handling
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			// Go back one level based on current state
			switch m.state {
			case stateMovieList:
				// From search results back to search input
				m.state = stateSearchInput
				m.textInput.Focus()
				m.results = nil
				m.selectedIdx = 0
				return m, textinput.Blink
			case stateMovieDetails:
				// From movie details back to search results if we came from search
				if m.mainPage == 0 {
					m.state = stateMovieList 
					m.movie = nil
					return m, nil
				} else {
					// From movie details back to list details if we came from a list
					m.state = stateViewListDetails
					m.movie = nil
					return m, nil
				}
			case stateCreateList:
				// From create list back to list management (base page)
				m.state = stateListManagement
				m.selectedIdx = 0
				return m, nil
			case stateViewListDetails:
				// From list details back to list overview
				m.state = stateListManagement
				m.selectedIdx = 0
				return m, nil
			case stateListSelection:
				// From list selection back to movie details
				m.state = stateMovieDetails
				return m, nil
			}
			// For base pages (stateSearchInput, stateListManagement), do nothing
		case "ctrl+l":
				// Toggle between search and list management
				if m.state == stateSearchInput || m.state == stateSearching || 
					m.state == stateMovieList || m.state == stateMovieDetails {
					// From search pages to list management
					m.state = stateListManagement
					return m, nil
				} else if m.state == stateListManagement || m.state == stateCreateList || 
					m.state == stateViewListDetails || m.state == stateListSelection {
					// From list pages to search input
					m.state = stateSearchInput
					m.textInput.Focus()
					return m, textinput.Blink
				}
		}

		// State-specific key handling
		switch m.state {
		case stateSearchInput:
			switch msg.String() {
			case "enter":
				if m.textInput.Value() != "" {
					m.state = stateSearching
					return m, tea.Batch(
						m.spinner.Tick,
						func() tea.Msg {
							results, err := m.api.SearchMovies(m.textInput.Value())
							if err != nil {
								return errorMsg{err}
							}
							return searchResultsMsg{results}
						},
					)
				}
			}

			// Handle text input
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)

		case stateMovieList:
			switch msg.String() {
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
			case "down", "j":
				if m.selectedIdx < len(m.results)-1 {
					m.selectedIdx++
				}
			case "enter":
				if len(m.results) > 0 {
					m.state = stateLoadingMovie
					selectedMovie := m.results[m.selectedIdx]
					return m, tea.Batch(
						m.spinner.Tick,
						func() tea.Msg {
							movie, err := m.api.GetMovieDetails(selectedMovie.URL)
							if err != nil {
								return errorMsg{err}
							}
							return movieDetailsMsg{movie}
						},
					)
				}
			}

		case stateListManagement:
			switch msg.String() {
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
			case "down", "j":
				// Allow navigating to the "Add New List" option at the bottom
				listCount := len(m.movieLists)
				if m.selectedIdx < listCount {
					m.selectedIdx++
				}
			case "enter":
				// View selected list or create a new list
				listCount := len(m.movieLists)
				if m.selectedIdx == listCount {
					// Selected "Add New List" option
					m.state = stateCreateList
					m.textInput.Placeholder = "Enter list name..."
					m.textInput.SetValue("")
					m.textInput.Focus()
					return m, textinput.Blink
				} else {
					// View selected list
					listNames := []string{}
					for listName := range m.movieLists {
						listNames = append(listNames, listName)
					}
					// Sort list names to ensure consistent order
					sort.Strings(listNames)
					
					if m.selectedIdx < len(listNames) {
						m.selectedList = listNames[m.selectedIdx]
						m.state = stateViewListDetails
						m.selectedIdx = 0
					}
				}
			}

		case stateCreateList:
			switch msg.String() {
			case "enter":
				// Create a new list with the entered name
				listName := m.textInput.Value()
				if listName != "" {
					if _, exists := m.movieLists[listName]; !exists {
						m.movieLists[listName] = []*Movie{}
					}
					m.state = stateListManagement
					m.textInput.SetValue("")
				}
				return m, nil
			}

			// Handle text input
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)

		case stateViewListDetails:
			switch msg.String() {
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
			case "down", "j":
				if m.selectedIdx < len(m.movieLists[m.selectedList])-1 {
					m.selectedIdx++
				}
			case "enter":
				// View selected movie details
				if len(m.movieLists[m.selectedList]) > 0 {
					m.movie = m.movieLists[m.selectedList][m.selectedIdx]
					m.state = stateMovieDetails

					// Set up the viewport with movie details content
					m.viewport.SetContent(m.formatMovieDetails())
				}
			case "d":
				// Delete selected movie from list
				if len(m.movieLists[m.selectedList]) > 0 {
					// Remove the movie from the list
					m.movieLists[m.selectedList] = append(
						m.movieLists[m.selectedList][:m.selectedIdx],
						m.movieLists[m.selectedList][m.selectedIdx+1:]...,
					)

					// Adjust selected index if needed
					if m.selectedIdx >= len(m.movieLists[m.selectedList]) {
						m.selectedIdx = max(0, len(m.movieLists[m.selectedList])-1)
					}

					// Save updated lists
					cmds = append(cmds, func() tea.Msg {
						if err := SaveMovieLists(m.movieLists); err != nil {
							return errorMsg{fmt.Errorf("failed to save movie lists: %v", err)}
						}
						return nil
					})
				}
			}

		case stateMovieDetails:
			switch msg.String() {
			case "enter":
				// Open the movie URL in browser when Enter is pressed
				if m.movie != nil && m.movie.URL != "" {
					return m, tea.Batch(
						func() tea.Msg {
							err := openBrowser(m.movie.URL)
							if err != nil {
								return errorMsg{fmt.Errorf("failed to open browser: %v", err)}
							}
							return openBrowserMsg{success: true}
						},
					)
				}
			case "alt+enter", "option+enter":
				// Search for movie trailer on YouTube
				if m.movie != nil {
					trailerSearchURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s+trailer",
						url.QueryEscape(m.movie.Title+" "+m.movie.Year))
					return m, tea.Batch(
						func() tea.Msg {
							err := openBrowser(trailerSearchURL)
							if err != nil {
								return errorMsg{fmt.Errorf("failed to open browser: %v", err)}
							}
							return youtubeSearchMsg{success: true}
						},
					)
				}
			case "s":
				// Save movie to a list
				if m.movie != nil {
					// If no lists exist yet, go to create list screen
					if len(m.movieLists) == 0 {
						m.state = stateCreateList
						m.textInput.Placeholder = "Enter list name to save movie..."
						m.textInput.SetValue("")
						m.textInput.Focus()
						return m, textinput.Blink
					} else {
						// Show list selection UI
						m.state = stateListSelection
						m.selectedIdx = 0
						return m, nil
					}
				}
			}
			// Handle viewport scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)

		case stateListSelection:
			switch msg.String() {
			case "up", "k":
				if m.selectedIdx > 0 {
					m.selectedIdx--
				}
			case "down", "j":
				if m.selectedIdx < len(m.movieLists)-1 {
					m.selectedIdx++
				}
			case "enter":
				// Save movie to the selected list
				if m.movie != nil {
					i := 0
					for listName := range m.movieLists {
						if i == m.selectedIdx {
							// Check if movie is already in the list
							alreadyExists := false
							for _, savedMovie := range m.movieLists[listName] {
								if savedMovie.URL == m.movie.URL {
									alreadyExists = true
									break
								}
							}

							if !alreadyExists {
								m.movieLists[listName] = append(m.movieLists[listName], m.movie)
							}

							// Save updated lists
							cmds = append(cmds, func() tea.Msg {
								if err := SaveMovieLists(m.movieLists); err != nil {
									return errorMsg{fmt.Errorf("failed to save movie lists: %v", err)}
								}
								return nil
							})

							break
						}
						i++
					}
				}
				m.state = stateMovieDetails
				return m, nil
			case "esc":
				// Cancel list selection
				m.state = stateMovieDetails
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 8

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case searchResultsMsg:
		m.results = msg.results
		if len(m.results) > 0 {
			m.state = stateMovieList
		} else {
			m.error = fmt.Errorf("no results found for \"%s\"", m.textInput.Value())
			m.state = stateError
			// After a delay, go back to search input
			cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
				return resetMsg{}
			}))
		}

	case movieDetailsMsg:
		m.movie = msg.movie
		m.state = stateMovieDetails

		// Set up the viewport with movie details content
		m.viewport.SetContent(m.formatMovieDetails())

	case openBrowserMsg:
		// We could show a notification that the browser was opened, but for now just continue

	case errorMsg:
		m.error = msg.err
		m.state = stateError
		// After a delay, go back to search input
		cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return resetMsg{}
		}))

	case resetMsg:
		// Reset to search input state
		m.state = stateSearchInput
		m.textInput.Focus()
		m.error = nil

	case listsLoadedMsg:
		// Replace the current movie lists with the loaded ones
		m.movieLists = msg.lists
		return m, nil

	case youtubeSearchMsg:
		// Message received when YouTube search was triggered
		// We could display a notification, but for now just continue
		return m, nil
	}

	// Update the paginator based on the current state
	if m.state == stateSearchInput || m.state == stateSearching || m.state == stateMovieList || m.state == stateMovieDetails {
		m.paginator.Page = 0 // Search page
		m.mainPage = 0
	} else if m.state == stateListManagement || m.state == stateCreateList || m.state == stateViewListDetails || m.state == stateListSelection {
		m.paginator.Page = 1 // Lists page
		m.mainPage = 1
	}

	return m, tea.Batch(cmds...)
}

// View renders the current UI
func (m Model) View() string {
	var sb strings.Builder

	// For calculating content height and footer position
	contentHeight := 0
	
	// Simple header without a box - much more resilient to window resizing
	headerText := "🍅 Rotten Tomatoes CLI"
	if m.width < len(headerText)+4 {
		headerText = "🍅 RT CLI" // Shorter version for very small windows
	}

	// Center the header text manually without using a fixed-width box
	headerStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Bold(true).
		Foreground(primaryColor).
		MarginBottom(1)
		
	sb.WriteString(headerStyle.Render(headerText))
	sb.WriteString("\n")
	
	// Container for the main content
	contentStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center)
	
	// Content builder for the main section
	var content strings.Builder

	switch m.state {
	case stateSearchInput:
		content.WriteString(subtitleStyle.Render("Search for a movie:"))
		content.WriteString("\n")
		
		// Center the input box
		inputWrapper := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
		content.WriteString(inputWrapper.Render(inputStyle.Render(m.textInput.View())))
		
		content.WriteString("\n\n")
		content.WriteString(normalTextStyle.Render("Press Enter to search, Ctrl+C to quit"))
		contentHeight += 6 // Approximate height

	case stateSearching:
		content.WriteString(subtitleStyle.Render("Searching..."))
		content.WriteString("\n")
		
		searchingStatus := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
		content.WriteString(searchingStatus.Render(
			m.spinner.View() + " " + "Looking for \"" + m.textInput.Value() + "\"",
		))
		contentHeight += 4

	case stateMovieList:
		content.WriteString(subtitleStyle.Render("Search Results:"))
		content.WriteString("\n")

		// Render movie list with selection
		var listContent strings.Builder
		for i, movie := range m.results {
			item := fmt.Sprintf("%s (%s)", movie.Title, movie.Year)
			if i == m.selectedIdx {
				listContent.WriteString(highlightedTextStyle.Render(item))
			} else {
				listContent.WriteString(normalTextStyle.Render("  " + item))
			}
			listContent.WriteString("\n")
		}
		
		// Center the movie list box
		listWrapper := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
		content.WriteString(listWrapper.Render(movieListStyle.Render(listContent.String())))
		
		content.WriteString("\n\n")
		content.WriteString(normalTextStyle.Render("↑/↓: Navigate • Enter: Select • Esc: Back • q: Quit"))
		contentHeight += len(m.results) + 6

	case stateLoadingMovie:
		content.WriteString(subtitleStyle.Render("Loading movie details..."))
		content.WriteString("\n")
		
		loadingStyle := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
		content.WriteString(loadingStyle.Render(
			m.spinner.View() + " " + "Fetching information for \"" + m.results[m.selectedIdx].Title + "\"",
		))
		contentHeight += 4

	case stateMovieDetails:
		// Movie details are already centered by formatMovieDetails
		content.WriteString(m.viewport.View())
		content.WriteString("\n\n")
		content.WriteString(normalTextStyle.Render("↑/↓: Scroll • Enter: Open in Browser • Alt+Enter: Search Trailer • s: Save to List • Esc: Back to results • q: Quit"))
		contentHeight += strings.Count(m.viewport.View(), "\n") + 4

	case stateListManagement:
		content.WriteString(subtitleStyle.Render("Your Movie Lists"))
		content.WriteString("\n\n")

		// Create a list of all lists plus an "Add List" option at the bottom
		var listContent strings.Builder
		var listNames []string
		
		// Get all list names for consistent ordering
		for listName := range m.movieLists {
			listNames = append(listNames, listName)
		}
		
		 // Sort list names alphabetically for consistent display order
		sort.Strings(listNames)
		
		// If there are no lists, just show the "Add List" option
		if len(listNames) == 0 {
			if m.selectedIdx == 0 {
				listContent.WriteString(highlightedTextStyle.Render("> + Add New List"))
			} else {
				listContent.WriteString(normalTextStyle.Render("  + Add New List"))
			}
		} else {
			// Display all existing lists in sorted order
			for i, listName := range listNames {
				movies := m.movieLists[listName]
				if i == m.selectedIdx {
					listContent.WriteString(highlightedTextStyle.Render(fmt.Sprintf("> %s (%d movies)", listName, len(movies))))
				} else {
					listContent.WriteString(normalTextStyle.Render(fmt.Sprintf("  %s (%d movies)", listName, len(movies))))
				}
				listContent.WriteString("\n")
			}
			
			// Add the "Add List" option at the bottom
			if len(listNames) == m.selectedIdx {
				listContent.WriteString(highlightedTextStyle.Render("> + Add New List"))
			} else {
				listContent.WriteString(normalTextStyle.Render("  + Add New List"))
			}
		}
		
		// Center the list box
		listWrapper := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
		content.WriteString(listWrapper.Render(movieListStyle.Render(listContent.String())))
		
		content.WriteString("\n\n")
		content.WriteString(normalTextStyle.Render("↑/↓: Navigate • Enter: Select • Ctrl+L: Go to Search"))
		contentHeight += len(listNames) + 7 // listNames + addList + margins

	case stateCreateList:
		content.WriteString(subtitleStyle.Render("Create a New List"))
		content.WriteString("\n")
		
		// Center the input box
		inputWrapper := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
		content.WriteString(inputWrapper.Render(inputStyle.Render(m.textInput.View())))
		
		content.WriteString("\n\n")
		content.WriteString(normalTextStyle.Render("Enter list name and press Enter • Esc: Cancel"))
		contentHeight += 6

	case stateViewListDetails:
		content.WriteString(subtitleStyle.Render(fmt.Sprintf("Movies in List: %s", m.selectedList)))
		content.WriteString("\n\n")

		if len(m.movieLists[m.selectedList]) == 0 {
			content.WriteString(normalTextStyle.Render("This list is empty. Add movies to it!"))
			contentHeight += 4
		} else {
			var listContent strings.Builder
			for i, movie := range m.movieLists[m.selectedList] {
				item := fmt.Sprintf("%s (%s)", movie.Title, movie.Year)
				if i == m.selectedIdx {
					listContent.WriteString(highlightedTextStyle.Render("> " + item))
				} else {
					listContent.WriteString(normalTextStyle.Render("  " + item))
				}
				listContent.WriteString("\n")
			}
			
			// Center the list box
			listWrapper := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
			content.WriteString(listWrapper.Render(movieListStyle.Render(listContent.String())))
			
			content.WriteString("\n\n")
			content.WriteString(normalTextStyle.Render("↑/↓: Navigate • Enter: View movie • d: Delete movie • Esc: Back"))
			contentHeight += len(m.movieLists[m.selectedList]) + 6
		}

	case stateListSelection:
		content.WriteString(subtitleStyle.Render("Select a List to Save Movie"))
		content.WriteString("\n\n")

		if len(m.movieLists) == 0 {
			content.WriteString(normalTextStyle.Render("You don't have any lists yet. Create one first!"))
			contentHeight += 4
		} else {
			var listContent strings.Builder
			listNames := []string{}
			
			// Get sorted list names for consistent display
			for listName := range m.movieLists {
				listNames = append(listNames, listName)
			}
			sort.Strings(listNames)
			
			for i, listName := range listNames {
				if i == m.selectedIdx {
					listContent.WriteString(highlightedTextStyle.Render("> " + listName))
				} else {
					listContent.WriteString(normalTextStyle.Render("  " + listName))
				}
				listContent.WriteString("\n")
			}
			
			// Center the list box
			listWrapper := lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center)
			content.WriteString(listWrapper.Render(movieListStyle.Render(listContent.String())))
			
			content.WriteString("\n\n")
			content.WriteString(normalTextStyle.Render("↑/↓: Navigate • Enter: Save to list • Esc: Cancel"))
			contentHeight += len(listNames) + 6
		}

	case stateError:
		content.WriteString(errorStyle.Render("Error: " + m.error.Error()))
		content.WriteString("\n\n")
		content.WriteString(normalTextStyle.Render("Returning to search in a moment..."))
		contentHeight += 4
	}

	// Apply content styling
	sb.WriteString(contentStyle.Render(content.String()))
	
	// Calculate available space and position footer at fixed distance from bottom
	availableHeight := m.height - contentHeight - 2 // 2 for header
	footerDistanceFromBottom := 6 // Footer always 6 lines from bottom
	
	// Calculate vertical padding to place footer at consistent position
	verticalPadding := availableHeight - footerDistanceFromBottom
	if verticalPadding < 0 {
		verticalPadding = 0
	}
	
	// Add vertical padding
	sb.WriteString(strings.Repeat("\n", verticalPadding))
	
	// Create fixed-position footer
	var footer strings.Builder
	
	// Always show paginator in fixed position for all states
	// This ensures it never jumps around when switching pages
	paginatorStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center)
		
	// Render paginator dots
	paginatorView := m.paginator.View()
	footer.WriteString(paginatorStyle.Render(paginatorView))
	footer.WriteString("\n")
	
	// Add the current page name
	var pageName string
	if m.mainPage == 0 {
		pageName = "Search"
	} else {
		pageName = "Lists"
	}
	
	pageNameStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Bold(true).
		Width(m.width).
		Align(lipgloss.Center)
		
	footer.WriteString(pageNameStyle.Render(pageName))
	footer.WriteString("\n\n")
	
	// Add hotkey instruction for switching pages
	instructionStyle := lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center)
		
	footer.WriteString(instructionStyle.Render("Press Ctrl+L to switch between Search and Lists"))
	
	// Add the footer to the main content
	sb.WriteString(footer.String())
	
	// Final wrapper to ensure everything is centered
	finalView := lipgloss.NewStyle().
		Width(m.width).
		MaxHeight(m.height).
		AlignHorizontal(lipgloss.Center).
		Render(sb.String())
		
	return finalView
}

// Format movie details for display
func (m Model) formatMovieDetails() string {
	if m.movie == nil {
		return "No movie details available"
	}

	var sb strings.Builder

	// Calculate content width to fit in the middle of the screen
	contentWidth := min(m.width-20, 80) // Use 80 as max width or adjust based on screen width

	// Create centered content container
	container := lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 3).
		MarginLeft((m.width - contentWidth) / 2).
		MarginRight((m.width - contentWidth) / 2)

	// Inner content builder
	var content strings.Builder

	// Title and year with larger, bordered style
	titleBox := titleStyle.Copy().
		Width(contentWidth - 10).
		Render(m.movie.Title + " (" + m.movie.Year + ")")

	content.WriteString(titleBox)
	content.WriteString("\n\n")

	// Scores
	scoreHeader := subtitleStyle.Copy().
		Width(contentWidth - 10).
		Render("Ratings:")
	content.WriteString(scoreHeader)
	content.WriteString("\n")

	tomatometerScore := fmt.Sprintf("🍅 Tomatometer (Critics): %s%%", m.movie.CriticScore)
	audienceScore := fmt.Sprintf("🍿 Popcornometer (Audience): %s%%", m.movie.AudienceScore)

	content.WriteString(scoreStyle.Copy().Width(contentWidth - 10).Render(tomatometerScore))
	content.WriteString("\n")
	content.WriteString(scoreStyle.Copy().Width(contentWidth - 10).Render(audienceScore))
	content.WriteString("\n\n")

	// Critics consensus - properly wrapped
	content.WriteString(subtitleStyle.Copy().Width(contentWidth - 10).Render("Critics Consensus:"))
	content.WriteString("\n")

	// Wrap the consensus text to fit the viewport
	consensusWidth := contentWidth - 16
	if consensusWidth < 40 {
		consensusWidth = 40 // Minimum width for consensus
	}

	consensus := m.movie.Consensus
	wrapped := wrapText(consensus, consensusWidth)
	content.WriteString(normalTextStyle.Copy().Width(contentWidth - 10).Render(wrapped))
	content.WriteString("\n\n")

	// URL
	content.WriteString(subtitleStyle.Copy().Width(contentWidth - 10).Render("More Info:"))
	content.WriteString("\n")
	content.WriteString(normalTextStyle.Copy().Width(contentWidth - 10).Render(m.movie.URL))

	// Apply the container style to all content
	sb.WriteString(container.Render(content.String()))

	return sb.String()
}

// wrapText wraps text to fit within a given width
func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}

	var result strings.Builder
	var lineLength int

	words := strings.Fields(text)
	for i, word := range words {
		// Handle very long words
		if len(word) > width {
			// If at the beginning of a line, add the word with a break
			if lineLength == 0 {
				result.WriteString(word[:width-1] + "-")
				word = word[width-1:]
				lineLength = 0
				result.WriteString("\n")
			}

			// Break the long word
			for len(word) > width {
				result.WriteString(word[:width-1] + "-\n")
				word = word[width-1:]
				lineLength = 0
			}
		}

		// Check if adding this word would exceed the width
		if lineLength+len(word) > width {
			// Start a new line
			result.WriteString("\n")
			lineLength = 0
		} else if i > 0 && lineLength > 0 {
			// Add a space before the word (unless it's the first word on a line)
			result.WriteString(" ")
			lineLength++
		}

		result.WriteString(word)
		lineLength += len(word)
	}

	return result.String()
}

// Helper function for max (Go <1.21 compatibility)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Helper function for min (Go <1.21 compatibility)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Custom message types
type searchResultsMsg struct {
	results []SearchResult
}

type movieDetailsMsg struct {
	movie *Movie
}

type errorMsg struct {
	err error
}

type resetMsg struct{}

type openBrowserMsg struct {
	success bool
}

type youtubeSearchMsg struct {
	success bool
}

type listsLoadedMsg struct {
	lists map[string][]*Movie
}

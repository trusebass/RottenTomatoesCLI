package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
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
	bgColor        = lipgloss.Color("#171717") // Near black

	// Text styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	normalTextStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	highlightedTextStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	scoreStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

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
)

// Model represents the application state
type Model struct {
	state       int
	api         *RottenTomatoesAPI
	textInput   textinput.Model
	spinner     spinner.Model
	results     []SearchResult
	selectedIdx int
	movie       *Movie
	error       error
	viewport    viewport.Model
	width       int
	height      int
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
	vp.Style = lipgloss.NewStyle().BorderForeground(accentColor)

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
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
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
			// Go back to search input from search results or movie details
			if m.state == stateMovieList || m.state == stateMovieDetails {
				m.state = stateSearchInput
				m.textInput.Focus()
				m.results = nil
				m.selectedIdx = 0
				m.movie = nil
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
			}
			// Handle viewport scrolling
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
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
	}

	return m, tea.Batch(cmds...)
}

// View renders the current UI
func (m Model) View() string {
	var sb strings.Builder

	sb.WriteString(titleStyle.Render("🍅 Rotten Tomatoes CLI"))
	sb.WriteString("\n\n")

	switch m.state {
	case stateSearchInput:
		sb.WriteString(subtitleStyle.Render("Search for a movie:"))
		sb.WriteString("\n")
		sb.WriteString(inputStyle.Render(m.textInput.View()))
		sb.WriteString("\n\n")
		sb.WriteString(normalTextStyle.Render("Press Enter to search, Ctrl+C to quit"))

	case stateSearching:
		sb.WriteString(subtitleStyle.Render("Searching..."))
		sb.WriteString("\n")
		sb.WriteString(m.spinner.View())
		sb.WriteString(" ")
		sb.WriteString(normalTextStyle.Render("Looking for \"" + m.textInput.Value() + "\""))

	case stateMovieList:
		sb.WriteString(subtitleStyle.Render("Search Results:"))
		sb.WriteString("\n")

		// Render movie list with selection
		var listContent strings.Builder
		for i, movie := range m.results {
			item := fmt.Sprintf("%s (%s)", movie.Title, movie.Year)
			if i == m.selectedIdx {
				listContent.WriteString(highlightedTextStyle.Render("> " + item))
			} else {
				listContent.WriteString(normalTextStyle.Render("  " + item))
			}
			listContent.WriteString("\n")
		}

		sb.WriteString(movieListStyle.Render(listContent.String()))
		sb.WriteString("\n\n")
		sb.WriteString(normalTextStyle.Render("↑/↓: Navigate • Enter: Select • Esc: Back • q: Quit"))

	case stateLoadingMovie:
		sb.WriteString(subtitleStyle.Render("Loading movie details..."))
		sb.WriteString("\n")
		sb.WriteString(m.spinner.View())
		sb.WriteString(" ")
		sb.WriteString(normalTextStyle.Render("Fetching information for \"" + m.results[m.selectedIdx].Title + "\""))

	case stateMovieDetails:
		sb.WriteString(m.viewport.View())
		sb.WriteString("\n\n")
		sb.WriteString(normalTextStyle.Render("↑/↓: Scroll • Enter: Open in Browser • Esc: Back to results • q: Quit"))

	case stateError:
		sb.WriteString(errorStyle.Render("Error: " + m.error.Error()))
		sb.WriteString("\n\n")
		sb.WriteString(normalTextStyle.Render("Returning to search in a moment..."))
	}

	// Add help text at the bottom
	return lipgloss.NewStyle().
		Width(m.width).
		AlignHorizontal(lipgloss.Center).
		MaxHeight(m.height).
		Render(sb.String())
}

// Format movie details for display
func (m Model) formatMovieDetails() string {
	if m.movie == nil {
		return "No movie details available"
	}

	var sb strings.Builder

	// Title and year
	sb.WriteString(titleStyle.Render(m.movie.Title + " (" + m.movie.Year + ")"))
	sb.WriteString("\n\n")

	// Scores
	scoreHeader := subtitleStyle.Render("Ratings:")
	sb.WriteString(scoreHeader)
	sb.WriteString("\n")

	tomatometerScore := fmt.Sprintf("Tomatometer (Critics): %s%%", m.movie.CriticScore)
	audienceScore := fmt.Sprintf("Popcornometer (Audience): %s%%", m.movie.AudienceScore)

	sb.WriteString(scoreStyle.Render(tomatometerScore))
	sb.WriteString("\n")
	sb.WriteString(scoreStyle.Render(audienceScore))
	sb.WriteString("\n\n")

	// Critics consensus - properly wrapped
	sb.WriteString(subtitleStyle.Render("Critics Consensus:"))
	sb.WriteString("\n")

	// Format the consensus text to ensure it wraps properly
	// Limit the line width to viewport width minus padding
	maxWidth := m.viewport.Width - 8
	if maxWidth < 20 {
		maxWidth = 60 // Fallback if viewport width is too small
	}

	// Wrap the consensus text to fit the viewport
	consensus := m.movie.Consensus
	// Handle long words by inserting spaces if needed
	wrapped := wrapText(consensus, maxWidth)
	sb.WriteString(normalTextStyle.Render(wrapped))
	sb.WriteString("\n\n")

	// URL
	sb.WriteString(subtitleStyle.Render("More Info:"))
	sb.WriteString("\n")
	sb.WriteString(normalTextStyle.Render(m.movie.URL))

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

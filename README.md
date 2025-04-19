# RT-TUI: Rotten Tomatoes Terminal UI

A Go-based terminal user interface for searching and retrieving movie ratings from Rotten Tomatoes.

![RT-TUI Screenshot](screenshot.png) <!-- Add a screenshot of your TUI here if available -->

## Features

- Search for movies by title
- Beautiful terminal UI with keyboard navigation
- Display movie details including critic and audience scores
- Fast and lightweight with minimal dependencies

## Installation

### From Source

1. Ensure you have Go 1.18 or later installed
2. Clone this repository
3. Build the application:

```bash
cd rt-tui
go build
```

4. Run the application:

```bash
./rt-tui
```

## Usage

1. Launch the application
2. Enter a movie title in the search box
3. Navigate through the search results using arrow keys
4. Press Enter to view detailed information about the selected movie

## Project Structure

- `rt-tui/` - Main Go implementation of the terminal UI
  - `main.go` - Entry point of the application
  - `ui.go` - Terminal UI implementation
  - `api.go` - API client for Rotten Tomatoes

- `RottenTomatoesCLIRawPython/` - Original Python prototype

## Dependencies

- [tcell](https://github.com/gdamore/tcell) - Terminal cell handling library
- [tview](https://github.com/rivo/tview) - Terminal UI library
- Go standard libraries

## Development

This project is actively under development. Contributions are welcome!

To contribute:
1. Fork the repository
2. Create a feature branch
3. Submit a pull request

## License

MIT License

## Acknowledgements

- [Rotten Tomatoes](https://www.rottentomatoes.com/) for the movie data
- The authors and maintainers of the Go libraries used in this project
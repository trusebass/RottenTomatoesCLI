# Contributing to RT-TUI (yapped together by Claude)

Thank you for considering contributing to RT-TUI! This document provides guidelines and instructions for contributing.


## How Can I Contribute?

### Reporting Bugs

- Check if the bug has already been reported in the Issues section
- Use the bug report template if available
- Include detailed steps to reproduce the bug
- Mention your operating system and Go version

### Suggesting Features

- Check if the feature has already been suggested in the Issues section
- Provide a clear description of the feature and its benefits
- Consider how the feature fits with the project's scope and goals

### Code Contributions

1. Fork the repository
2. Create a new branch for your feature or bug fix
3. Make your changes
4. Run tests if available
5. Submit a pull request

## Pull Request Process

1. Update the README.md if needed (e.g., new features, changed commands)
2. Update documentation for any changed functionality
3. Ensure your code follows the project's style and conventions
4. Reference any relevant issues in your pull request description

## Development Setup

1. Ensure you have Go 1.18 or later installed
2. Clone your fork of the repository
3. Install dependencies:
   ```bash
   cd rt-tui
   go mod download
   ```
4. Build and run the application:
   ```bash
   go build -o rotten
   ./rotten
   ```

## Style Guidelines

- Follow standard Go code formatting (run `go fmt` before committing)
- Use meaningful variable and function names
- Add comments for complex logic
- Keep functions small and focused

## Questions?

If you have any questions about contributing, feel free to open an issue asking for clarification.
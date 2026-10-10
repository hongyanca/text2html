# Text to HTML

A small Go web application that accepts a plain-text upload and produces a standalone HTML5 document. Source text is escaped before it is written into paragraph elements, so uploaded markup is displayed as text.

## Requirements

- Go 1.26 or newer
- A modern browser with JavaScript enabled

## Project structure

```text
.
├── main.go             # Server startup and routes
├── server.go           # Upload handling, jobs, progress, result delivery
├── converter.go        # Paragraph detection, splitting, and HTML generation
├── id.go               # Cryptographically random job IDs
├── config.toml         # Data directory, HTTP port, and paragraph word limit
├── converter_test.go   # Conversion unit tests
├── web/
│   ├── index.html
│   ├── style.css
│   └── app.js
├── go.mod
└── README.md
```

The frontend assets are embedded into the executable with `embed`.

Use the **Delete all generated HTML files** button to remove generated output from the configured data directory. Active conversions are allowed to finish.

## Run

```sh
go run .
```

Open [http://localhost:8080](http://localhost:8080); the server prints this access link at startup. Edit `config.toml` to change the HTTP port, storage directory, or maximum generated paragraph length. `max_paragraph_words` defaults to 300 and must be a positive integer. By default, uploads and generated HTML files are stored in `/Volumes/RAM-Disk/text2html`. The application creates this directory at startup if it does not exist.

## Tests

```sh
go test ./...
```

## Limits and behavior

Uploads must have a `.txt` extension and be no larger than 5 MiB. Each job receives an unguessable random identifier; its input is removed after conversion, and inputs/results and in-memory job records are cleaned up after 24 hours. Results are stored with restrictive file permissions in the application data directory. The generated filename uses the uploaded base name with `.html` in place of `.txt`.

Paragraphs separated by one or more blank lines remain separate. CRLF and CR line endings are normalized to Unix newlines. If blank-line separators are absent, wrapped lines are joined with spaces; a line ending in a sentence-ending mark is considered a likely paragraph boundary. Paragraphs above `max_paragraph_words` are split at the last sentence-ending period before the limit, or at the word boundary when no such period exists. Splitting continues until every paragraph is within the configured word limit. The browser displays upload progress and polls the server for actual conversion stages.

## Example

Input:

```text
Hello, world!

This is a second paragraph.
```

The body of the generated document is:

```html
<p>Hello, world!</p>
<p>This is a second paragraph.</p>
```

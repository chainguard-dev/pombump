package pkg

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxFileSize   = 100 * 1024 * 1024 // 100MB
	maxLineLength = 1024 * 1024       // 1MB per line
)

// CommentBlock represents a comment with its location context
type CommentBlock struct {
	Content      []string // The comment lines
	BeforeXPath  string   // Simplified XPath-like location (e.g., "/project/properties/slf4j.version")
	AfterXPath   string   // For comments that come after an element
	LineNumber   int      // Original line number for ordering
	IsInline     bool     // True if comment is on same line as XML element
	IndentLevel  int      // Indentation level of the comment
}

// PreserveCommentsInPOMUpdate preserves all comments from the original POM file
func PreserveCommentsInPOMUpdate(inputPath string, outputContent []byte) ([]byte, error) {
	// Validate and clean the input path
	cleanPath := filepath.Clean(inputPath)
	if err := ValidateFilePath(cleanPath); err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}

	// Check file size before reading
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if fileInfo.Size() > maxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes (max: %d)", fileInfo.Size(), maxFileSize)
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()

	// Parse original file to extract all comments with their context
	comments, err := extractAllComments(file)
	if err != nil {
		return nil, err
	}

	// Handle empty output
	if len(outputContent) == 0 {
		return []byte{}, nil
	}

	// Parse output to determine where to insert comments
	result, err := insertComments(string(outputContent), comments)
	if err != nil {
		return nil, err
	}

	return []byte(result), nil
}

// extractAllComments parses the XML file and extracts all comments with their context
func extractAllComments(reader io.Reader) ([]CommentBlock, error) {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLineLength)

	var comments []CommentBlock
	var currentPath []string // Track current XML path
	var lines []string
	lineNum := 0
	
	// Read all lines first
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		lineNum++
		if lineNum > 100000 {
			return nil, fmt.Errorf("file has too many lines (max: 100000)")
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Parse lines to extract comments and their context
	inComment := false
	var currentComment []string
	commentStartLine := 0
	
	// Regular expressions for XML parsing
	openTagRe := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9._-]*)[^>]*>`)
	closeTagRe := regexp.MustCompile(`</([a-zA-Z][a-zA-Z0-9._-]*)>`)
	selfClosingRe := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9._-]*)[^>]*/\s*>`)
	
	// Track which line has the project closing tag
	projectCloseLineNum := -1
	for i, line := range lines {
		if strings.Contains(line, "</project>") {
			projectCloseLineNum = i
			break
		}
	}
	
	// Track comments that appear before any XML content
	var preXMLComments []CommentBlock
	foundXMLDeclaration := false
	
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check if we've found the XML declaration
		if strings.HasPrefix(trimmed, "<?xml") {
			foundXMLDeclaration = true
		}
		
		// Handle multi-line comments
		if strings.Contains(line, "<!--") {
			inComment = true
			commentStartLine = i
			currentComment = []string{}
			
			// Calculate indent level
			indentLevel := len(line) - len(strings.TrimLeft(line, " \t"))
			
			// Check if it's a single-line comment
			if strings.Contains(line, "-->") {
				currentComment = append(currentComment, line)
				
				// Skip inline comments (comments on same line as XML elements)
				if hasNonCommentContent(line) {
					inComment = false
					currentComment = nil
					continue
				}
				
				// Determine context
				comment := CommentBlock{
					Content:     currentComment,
					LineNumber:  commentStartLine,
					IsInline:    false,
					IndentLevel: indentLevel,
				}
				
				// If this comment appears before XML declaration, store it separately
				if !foundXMLDeclaration {
					preXMLComments = append(preXMLComments, comment)
				} else {
					// Check if this is after the project closing tag
					if projectCloseLineNum > 0 && commentStartLine > projectCloseLineNum {
						comment.AfterXPath = "END_OF_FILE"
					} else {
						// Set the path context
						if len(currentPath) > 0 {
							comment.AfterXPath = "/" + strings.Join(currentPath, "/")
						}
						
						// Look ahead for the next element
						if i+1 < len(lines) {
							nextLine := strings.TrimSpace(lines[i+1])
							if matches := openTagRe.FindStringSubmatch(nextLine); len(matches) > 1 {
								tagName := matches[1]
								if tagName != "relativePath" { // Skip relativePath as it's often self-closing
									comment.BeforeXPath = "/" + strings.Join(append(currentPath, tagName), "/")
								}
							}
						}
					}
					
					comments = append(comments, comment)
				}
				inComment = false
				currentComment = nil
			} else {
				currentComment = append(currentComment, line)
			}
			continue
		}
		
		if inComment {
			currentComment = append(currentComment, line)
			if strings.Contains(line, "-->") {
				// End of multi-line comment
				// Calculate indent level from first line
				indentLevel := len(currentComment[0]) - len(strings.TrimLeft(currentComment[0], " \t"))
				
				comment := CommentBlock{
					Content:     currentComment,
					LineNumber:  commentStartLine,
					IsInline:    false,
					IndentLevel: indentLevel,
				}
				
				// If this comment appears before XML declaration, store it separately
				if !foundXMLDeclaration {
					preXMLComments = append(preXMLComments, comment)
				} else {
					// Check if this is after the project closing tag
					if projectCloseLineNum > 0 && commentStartLine > projectCloseLineNum {
						comment.AfterXPath = "END_OF_FILE"
					} else {
						// Set the path context
						if len(currentPath) > 0 {
							comment.AfterXPath = "/" + strings.Join(currentPath, "/")
						}
						
						// Look ahead for the next element
						if i+1 < len(lines) {
							nextLine := strings.TrimSpace(lines[i+1])
							if matches := openTagRe.FindStringSubmatch(nextLine); len(matches) > 1 {
								tagName := matches[1]
								comment.BeforeXPath = "/" + strings.Join(append(currentPath, tagName), "/")
							}
						}
					}
					
					comments = append(comments, comment)
				}
				inComment = false
				currentComment = nil
			}
			continue
		}
		
		// Track XML path for context
		// Handle self-closing tags
		if matches := selfClosingRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			// Self-closing tag, don't add to path
			continue
		}
		
		// Handle opening tags
		if matches := openTagRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			tagName := matches[1]
			// Skip XML declaration and comments
			if !strings.HasPrefix(tagName, "?") && !strings.HasPrefix(tagName, "!") {
				currentPath = append(currentPath, tagName)
			}
		}
		
		// Handle closing tags
		if matches := closeTagRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			tagName := matches[1]
			// Pop from path if it matches
			if len(currentPath) > 0 && currentPath[len(currentPath)-1] == tagName {
				currentPath = currentPath[:len(currentPath)-1]
			}
		}
	}
	
	// Prepend pre-XML comments
	return append(preXMLComments, comments...), nil
}

// hasNonCommentContent checks if a line has content other than a comment
func hasNonCommentContent(line string) bool {
	// Remove comment portion
	commentStart := strings.Index(line, "<!--")
	commentEnd := strings.Index(line, "-->")
	
	if commentStart == -1 || commentEnd == -1 {
		return false
	}
	
	before := strings.TrimSpace(line[:commentStart])
	after := strings.TrimSpace(line[commentEnd+3:])
	
	return before != "" || after != ""
}


// insertComments inserts comments back into the output XML at appropriate locations
func insertComments(output string, comments []CommentBlock) (string, error) {
	if len(comments) == 0 {
		return output, nil
	}
	
	lines := strings.Split(output, "\n")
	var result []string
	var currentPath []string
	
	// Regular expressions for XML parsing
	openTagRe := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9._-]*)[^>]*>`)
	closeTagRe := regexp.MustCompile(`</([a-zA-Z][a-zA-Z0-9._-]*)>`)
	selfClosingRe := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9._-]*)[^>]*/\s*>`)
	
	
	// Separate comments by type
	var preXMLComments []CommentBlock
	var endOfFileComments []CommentBlock
	var regularComments []CommentBlock
	
	for _, comment := range comments {
		if comment.BeforeXPath == "" && comment.AfterXPath == "" {
			// Comments before any XML content
			preXMLComments = append(preXMLComments, comment)
		} else if comment.AfterXPath == "END_OF_FILE" {
			// Comments after the closing project tag
			endOfFileComments = append(endOfFileComments, comment)
		} else {
			regularComments = append(regularComments, comment)
		}
	}
	
	// First, handle any comments that come before the XML declaration
	foundXMLDecl := false
	for i, line := range lines {
		if strings.Contains(line, "<?xml") {
			// Insert pre-XML comments before the declaration
			for _, comment := range preXMLComments {
				result = append(result, comment.Content...)
			}
			foundXMLDecl = true
			// Process the rest of the lines starting from XML declaration
			lines = lines[i:]
			break
		}
	}
	
	// If no XML declaration found, add pre-XML comments at the beginning
	if !foundXMLDecl && len(preXMLComments) > 0 {
		for _, comment := range preXMLComments {
			result = append(result, comment.Content...)
		}
	}
	
	// Track which comments have been inserted
	insertedComments := make(map[int]bool)
	
	// Count occurrences of each path to handle multiple elements with same tag
	pathCounts := make(map[string]int)
	currentPathCounts := make(map[string]int)
	
	// Process the main content
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Track current path
		if matches := selfClosingRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			// Self-closing tag, check for comments before it
			tagName := matches[1]
			currentFullPath := "/" + strings.Join(append(currentPath, tagName), "/")
			
			// Increment count for this path
			pathCounts[currentFullPath]++
			
			// Insert any comments that should come before this element
			// Only insert the first uninserted comment for this path
			for i, comment := range regularComments {
				if !insertedComments[i] && comment.BeforeXPath == currentFullPath && !comment.IsInline {
					// Always preserve original formatting exactly
					result = append(result, comment.Content...)
					insertedComments[i] = true
					break // Only insert one comment per element
				}
			}
		} else if matches := openTagRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			tagName := matches[1]
			if !strings.HasPrefix(tagName, "?") && !strings.HasPrefix(tagName, "!") {
				currentFullPath := "/" + strings.Join(append(currentPath, tagName), "/")
				
				// Increment count for this path
				currentPathCounts[currentFullPath]++
				
				// Insert any comments that should come before this element
				// Only insert the first uninserted comment for this path
				for i, comment := range regularComments {
					if !insertedComments[i] && comment.BeforeXPath == currentFullPath && !comment.IsInline {
						// Always preserve original formatting exactly
						result = append(result, comment.Content...)
						insertedComments[i] = true
						break // Only insert one comment per element
					}
				}
				
				currentPath = append(currentPath, tagName)
			}
		}
		
		// Add the current line
		result = append(result, line)
		
		// Check if we should add comments after this line
		
		// Special handling for project-level comments (like copyright)
		if strings.Contains(line, "<project") {
			for i, comment := range regularComments {
				if !insertedComments[i] && comment.AfterXPath == "/project" && !strings.Contains(comment.BeforeXPath, "/project/") {
					// For comments after project tag, preserve original formatting exactly
					result = append(result, comment.Content...)
					insertedComments[i] = true
				}
			}
		}
		
		// Handle closing tags
		if matches := closeTagRe.FindStringSubmatch(trimmed); len(matches) > 1 {
			tagName := matches[1]
			if len(currentPath) > 0 && currentPath[len(currentPath)-1] == tagName {
				currentPath = currentPath[:len(currentPath)-1]
			}
		}
	}
	
	// Add end-of-file comments at the very end
	for _, comment := range endOfFileComments {
		result = append(result, comment.Content...)
	}
	
	return strings.Join(result, "\n"), nil
}

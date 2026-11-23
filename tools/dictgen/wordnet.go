package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type posFile struct {
	path         string
	partOfSpeech string
}

// posFiles is the import order. It determines entry IDs, so it must stay
// stable across releases.
var posFiles = []struct {
	name         string
	partOfSpeech string
}{
	{name: "data.noun", partOfSpeech: "noun"},
	{name: "data.verb", partOfSpeech: "verb"},
	{name: "data.adj", partOfSpeech: "adjective"},
	{name: "data.adv", partOfSpeech: "adverb"},
}

// exceptionFiles is the WordNet irregular-form order.
var exceptionFiles = []struct {
	name         string
	partOfSpeech string
}{
	{name: "noun.exc", partOfSpeech: "noun"},
	{name: "verb.exc", partOfSpeech: "verb"},
	{name: "adj.exc", partOfSpeech: "adjective"},
	{name: "adv.exc", partOfSpeech: "adverb"},
}

func findPOSFiles(directory string) ([]posFile, error) {
	var files []posFile
	for _, candidate := range posFiles {
		path := filepath.Join(directory, candidate.name)
		info, err := os.Stat(path)
		switch {
		case err == nil && info.Mode().IsRegular():
			files = append(files, posFile{path: path, partOfSpeech: candidate.partOfSpeech})
		case err == nil:
			return nil, fmt.Errorf("%s is not a regular file", path)
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("inspect %s: %w", path, err)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no WordNet data files (data.noun, data.verb, data.adj, data.adv) found in %s", directory)
	}
	return files, nil
}

// parseSynsets calls emit for every WordNet synset line. The format is a
// space-separated header followed by a gloss after the first '|'.
func parseSynsets(path string, emit func(lemmas []string, definition string) error) error {
	//nolint:gosec // path is a build-time input chosen on the command line
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		// The first lines are an indented license header.
		if line == "" || strings.HasPrefix(line, "  ") {
			continue
		}
		header, gloss, found := strings.Cut(line, "|")
		if !found {
			continue
		}
		fields := strings.Fields(header)
		if len(fields) < 5 {
			continue
		}
		wordCount, err := strconv.ParseInt(fields[3], 16, 64)
		if err != nil {
			continue
		}
		lemmas := make([]string, 0, wordCount)
		index := 4
		for range wordCount {
			if index >= len(fields) {
				break
			}
			lemmas = append(lemmas, strings.ReplaceAll(strings.TrimSpace(fields[index]), "_", " "))
			index += 2
		}
		if len(lemmas) == 0 {
			continue
		}
		if err := emit(lemmas, cleanGloss(gloss)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// cleanGloss keeps the definition before any quoted example and normalizes its
// capitalization.
func cleanGloss(gloss string) string {
	definition := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(strings.SplitN(gloss, `"`, 2)[0]), ";"))
	if definition == "" {
		definition = strings.TrimSpace(strings.Trim(strings.TrimSpace(gloss), `"`))
	}
	if definition == "" {
		return ""
	}
	first, size := utf8.DecodeRuneInString(definition)
	return string(unicode.ToUpper(first)) + definition[size:]
}

// parseExceptionFile calls emit for every inflected/base pair in a WordNet
// *.exc file.
func parseExceptionFile(path string, emit func(inflected, base string) error) error {
	//nolint:gosec // path is a build-time input chosen on the command line
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return parseExceptionLines(file, emit)
}

// parseExceptionLines parses the same "inflected base1 base2..." format from
// any reader, including the embedded supplement.
func parseExceptionLines(reader io.Reader, emit func(inflected, base string) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		inflected := strings.ReplaceAll(fields[0], "_", " ")
		for _, base := range fields[1:] {
			if err := emit(inflected, strings.ReplaceAll(base, "_", " ")); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

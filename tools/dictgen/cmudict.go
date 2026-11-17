package main

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var cmudictVariant = regexp.MustCompile(`\(\d+\)$`)

// parseCMUDict returns lowercased headword to IPA. Alternate pronunciations are
// combined in file order with "; " before conversion, matching the reference
// importer.
func parseCMUDict(path string) (map[string]string, error) {
	//nolint:gosec // path is a build-time input chosen on the command line
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	pronunciations := map[string][]string{}
	var order []string

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";;;") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		word := cmudictVariant.ReplaceAllString(fields[0], "")
		word = strings.ToLower(word)
		if _, exists := pronunciations[word]; !exists {
			order = append(order, word)
		}
		pronunciations[word] = append(pronunciations[word], strings.Join(fields[1:], " "))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result := make(map[string]string, len(pronunciations))
	for _, word := range order {
		combined := strings.Join(pronunciations[word], " ; ")
		if ipa := arpabetToIPA(combined); ipa != "" {
			result[word] = ipa
		}
	}
	return result, nil
}

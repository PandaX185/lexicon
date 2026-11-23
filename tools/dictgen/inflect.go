package main

import (
	"strings"
	"unicode"
)

var pluralEsExceptions = map[string]struct{}{
	"go": {}, "do": {}, "echo": {}, "hero": {}, "potato": {},
	"tomato": {}, "veto": {}, "torpedo": {},
}

type generatedForm struct {
	form string
	kind string
}

type inflectionSegment struct {
	prefix string
	token  string
	suffix string
}

// generatedForms yields conservative regular inflections for one headword.
// Single words use every inflection allowed by their parts of speech. Phrases
// and hyphenated words get a plural for each inflectable segment, which covers
// English compounds such as "school buses" and "attorneys general" without
// inventing tenses for the whole phrase. WordNet's exception files supply the
// irregular cases.
func generatedForms(word string, parts map[string]struct{}) []generatedForm {
	segments := splitInflectionSegments(word)
	if len(segments) == 1 {
		return generatedFormsForToken(segments[0].token, parts)
	}

	var forms []generatedForm
	for _, segment := range segments {
		segmentForms := generatedFormsForToken(segment.token, nil)
		for _, form := range segmentForms {
			forms = append(forms, generatedForm{form: segment.prefix + form.form + segment.suffix, kind: form.kind})
		}
	}
	return forms
}

func splitInflectionSegments(word string) []inflectionSegment {
	var segments []inflectionSegment
	start := 0
	for index, character := range word {
		if character != ' ' && character != '-' {
			continue
		}
		if token := word[start:index]; token != "" {
			segments = append(segments, inflectionSegment{
				prefix: word[:start],
				token:  token,
				suffix: word[index:],
			})
		}
		start = index + 1
	}
	if token := word[start:]; token != "" {
		segments = append(segments, inflectionSegment{prefix: word[:start], token: token})
	}
	if len(segments) == 0 {
		segments = append(segments, inflectionSegment{token: word})
	}
	return segments
}

func generatedFormsForToken(token string, parts map[string]struct{}) []generatedForm {
	if !isInflectable(token) {
		return nil
	}

	var forms []generatedForm
	if parts == nil || containsPart(parts, "noun") {
		forms = append(forms, generatedForm{form: pluralOf(token), kind: "plural"})
	}
	if parts != nil {
		if containsPart(parts, "verb") {
			forms = append(forms,
				generatedForm{form: thirdPersonOf(token), kind: "third_person"},
				generatedForm{form: pastOf(token), kind: "past"},
				generatedForm{form: gerundOf(token), kind: "gerund"},
			)
		}
		if containsPart(parts, "adjective") {
			forms = append(forms,
				generatedForm{form: comparativeOf(token), kind: "comparative"},
				generatedForm{form: superlativeOf(token), kind: "superlative"},
			)
		}
	}
	return forms
}

func containsPart(parts map[string]struct{}, part string) bool {
	_, ok := parts[part]
	return ok
}

func isInflectable(token string) bool {
	characters := []rune(token)
	if len(characters) < 3 {
		return false
	}
	for _, character := range characters {
		if !unicode.IsLetter(character) {
			return false
		}
	}
	return true
}

func lowerRunes(word string) []rune {
	return []rune(strings.ToLower(word))
}

func isVowelRune(character rune) bool {
	switch character {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func vowelGroupCount(word string) int {
	count := 0
	previousWasVowel := false
	for _, character := range lowerRunes(word) {
		isVowel := isVowelRune(character)
		if isVowel && !previousWasVowel {
			count++
		}
		previousWasVowel = isVowel
	}
	return count
}

func endsWithConsonantVowelConsonant(word string) bool {
	characters := lowerRunes(word)
	if len(characters) < 3 {
		return false
	}
	first, middle, final := characters[len(characters)-3], characters[len(characters)-2], characters[len(characters)-1]
	return unicode.IsLetter(first) && !isVowelRune(first) &&
		isVowelRune(middle) &&
		unicode.IsLetter(final) && !isVowelRune(final) && final != 'w' && final != 'x' && final != 'y'
}

func lastTwoRunes(word string) (rune, rune) {
	characters := lowerRunes(word)
	if len(characters) == 0 {
		return 0, 0
	}
	if len(characters) == 1 {
		return 0, characters[0]
	}
	return characters[len(characters)-2], characters[len(characters)-1]
}

func repeatedLastRune(word string) string {
	characters := []rune(word)
	return string(characters[len(characters)-1:])
}

func pluralOf(word string) string {
	lower := strings.ToLower(word)
	characters := []rune(word)
	previous, last := lastTwoRunes(word)
	switch {
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "x"), strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"), strings.HasSuffix(lower, "sh"):
		return word + "es"
	case last == 'y' && len(characters) > 1 && !isVowelRune(previous):
		return string(characters[:len(characters)-1]) + "ies"
	}
	if _, ok := pluralEsExceptions[lower]; ok {
		return word + "es"
	}
	return word + "s"
}

func thirdPersonOf(word string) string {
	switch strings.ToLower(word) {
	case "have":
		return "has"
	case "be":
		return "is"
	}
	return pluralOf(word)
}

func pastOf(word string) string {
	previous, last := lastTwoRunes(word)
	switch {
	case last == 'e':
		return word + "d"
	case last == 'y' && len([]rune(word)) > 1 && !isVowelRune(previous):
		return string([]rune(word)[:len([]rune(word))-1]) + "ied"
	case endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1:
		return word + repeatedLastRune(word) + "ed"
	}
	return word + "ed"
}

func gerundOf(word string) string {
	lower := strings.ToLower(word)
	characters := []rune(word)
	switch {
	case strings.HasSuffix(lower, "ie"):
		return string(characters[:len(characters)-2]) + "ying"
	case strings.HasSuffix(lower, "e") && !strings.HasSuffix(lower, "ee") &&
		!strings.HasSuffix(lower, "ye") && !strings.HasSuffix(lower, "oe"):
		return string(characters[:len(characters)-1]) + "ing"
	case endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1:
		return word + repeatedLastRune(word) + "ing"
	}
	return word + "ing"
}

func comparativeOf(word string) string {
	characters := []rune(word)
	previous, last := lastTwoRunes(word)
	switch {
	case last == 'y' && len(characters) > 1 && !isVowelRune(previous):
		return string(characters[:len(characters)-1]) + "ier"
	case last == 'e':
		return word + "r"
	case endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1:
		return word + repeatedLastRune(word) + "er"
	}
	return word + "er"
}

func superlativeOf(word string) string {
	characters := []rune(word)
	previous, last := lastTwoRunes(word)
	switch {
	case last == 'y' && len(characters) > 1 && !isVowelRune(previous):
		return string(characters[:len(characters)-1]) + "iest"
	case last == 'e':
		return word + "st"
	case endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1:
		return word + repeatedLastRune(word) + "est"
	}
	return word + "est"
}

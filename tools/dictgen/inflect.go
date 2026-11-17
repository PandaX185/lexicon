package main

import "strings"

var pluralEsExceptions = map[string]struct{}{
	"go": {}, "do": {}, "echo": {}, "hero": {}, "potato": {},
	"tomato": {}, "veto": {}, "torpedo": {},
}

type generatedForm struct {
	form string
	kind string
}

// generatedForms yields conservative regular inflections for one headword. It
// deliberately skips short, non-ASCII, and non-alphabetic words; WordNet's
// exception files supply the irregular cases.
func generatedForms(word string, parts map[string]struct{}) []generatedForm {
	if len(word) < 3 || !isASCIIAlpha(word) {
		return nil
	}

	var forms []generatedForm
	if _, ok := parts["noun"]; ok {
		forms = append(forms, generatedForm{form: pluralOf(word), kind: "plural"})
	}
	if _, ok := parts["verb"]; ok {
		forms = append(forms,
			generatedForm{form: thirdPersonOf(word), kind: "third_person"},
			generatedForm{form: pastOf(word), kind: "past"},
			generatedForm{form: gerundOf(word), kind: "gerund"},
		)
	}
	if _, ok := parts["adjective"]; ok {
		forms = append(forms,
			generatedForm{form: comparativeOf(word), kind: "comparative"},
			generatedForm{form: superlativeOf(word), kind: "superlative"},
		)
	}
	return forms
}

func isASCIIAlpha(word string) bool {
	for i := 0; i < len(word); i++ {
		character := word[i]
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') {
			return false
		}
	}
	return true
}

func vowelGroupCount(word string) int {
	count := 0
	previousWasVowel := false
	for _, character := range strings.ToLower(word) {
		isVowel := character == 'a' || character == 'e' || character == 'i' || character == 'o' || character == 'u'
		if isVowel && !previousWasVowel {
			count++
		}
		previousWasVowel = isVowel
	}
	return count
}

func endsWithConsonantVowelConsonant(word string) bool {
	if len(word) < 3 {
		return false
	}
	last := strings.ToLower(word[len(word)-3:])
	first, middle, final := last[0], last[1], last[2]
	return isASCIILetter(first) && !isVowel(first) &&
		isVowel(middle) &&
		isASCIILetter(final) && !isVowel(final) && final != 'w' && final != 'x' && final != 'y'
}

func isASCIILetter(character byte) bool {
	return (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
}

func isVowel(character byte) bool {
	return character == 'a' || character == 'e' || character == 'i' || character == 'o' || character == 'u'
}

func pluralOf(word string) string {
	lower := strings.ToLower(word)
	switch {
	case strings.HasSuffix(lower, "s"), strings.HasSuffix(lower, "x"), strings.HasSuffix(lower, "z"),
		strings.HasSuffix(lower, "ch"), strings.HasSuffix(lower, "sh"):
		return word + "es"
	case strings.HasSuffix(lower, "y") && len(word) > 1 && !isVowel(lower[len(word)-2]):
		return word[:len(word)-1] + "ies"
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
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "e") {
		return word + "d"
	}
	if strings.HasSuffix(lower, "y") && len(word) > 1 && !isVowel(lower[len(word)-2]) {
		return word[:len(word)-1] + "ied"
	}
	if endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1 {
		return word + word[len(word)-1:] + "ed"
	}
	return word + "ed"
}

func gerundOf(word string) string {
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "ie") {
		return word[:len(word)-2] + "ying"
	}
	if strings.HasSuffix(lower, "e") && !strings.HasSuffix(lower, "ee") &&
		!strings.HasSuffix(lower, "ye") && !strings.HasSuffix(lower, "oe") {
		return word[:len(word)-1] + "ing"
	}
	if endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1 {
		return word + word[len(word)-1:] + "ing"
	}
	return word + "ing"
}

func comparativeOf(word string) string {
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "y") && len(word) > 1 && !isVowel(lower[len(word)-2]) {
		return word[:len(word)-1] + "ier"
	}
	if strings.HasSuffix(lower, "e") {
		return word + "r"
	}
	if endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1 {
		return word + word[len(word)-1:] + "er"
	}
	return word + "er"
}

func superlativeOf(word string) string {
	lower := strings.ToLower(word)
	if strings.HasSuffix(lower, "y") && len(word) > 1 && !isVowel(lower[len(word)-2]) {
		return word[:len(word)-1] + "iest"
	}
	if strings.HasSuffix(lower, "e") {
		return word + "st"
	}
	if endsWithConsonantVowelConsonant(word) && vowelGroupCount(word) == 1 {
		return word + word[len(word)-1:] + "est"
	}
	return word + "est"
}

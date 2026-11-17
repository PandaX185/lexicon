package main

import (
	"regexp"
	"strings"
)

var phonemeToIPA = map[string]string{
	"AA": "ɑ", "AE": "æ", "AH": "ʌ", "AO": "ɔ", "AW": "aʊ",
	"AY": "aɪ", "B": "b", "CH": "tʃ", "D": "d", "DH": "ð",
	"EH": "ɛ", "ER": "ɜːr", "EY": "eɪ", "F": "f", "G": "ɡ",
	"HH": "h", "IH": "ɪ", "IY": "iː", "JH": "dʒ", "K": "k",
	"L": "l", "M": "m", "N": "n", "NG": "ŋ", "OW": "oʊ",
	"OY": "ɔɪ", "P": "p", "R": "r", "S": "s", "SH": "ʃ",
	"T": "t", "TH": "θ", "UH": "ʊ", "UW": "uː", "V": "v",
	"W": "w", "Y": "j", "Z": "z", "ZH": "ʒ",
}

var ipaVowels = map[string]struct{}{
	"AA": {}, "AE": {}, "AH": {}, "AO": {}, "AW": {}, "AY": {},
	"EH": {}, "ER": {}, "EY": {}, "IH": {}, "IY": {}, "OW": {},
	"OY": {}, "UH": {}, "UW": {},
}

var validOnsetPairs = map[[2]string]struct{}{
	{"P", "R"}: {}, {"P", "L"}: {}, {"B", "R"}: {}, {"B", "L"}: {},
	{"T", "R"}: {}, {"T", "W"}: {}, {"D", "R"}: {}, {"D", "W"}: {},
	{"K", "R"}: {}, {"K", "L"}: {}, {"K", "W"}: {}, {"G", "R"}: {},
	{"G", "L"}: {}, {"G", "W"}: {}, {"F", "R"}: {}, {"F", "L"}: {},
	{"TH", "R"}: {}, {"TH", "W"}: {}, {"S", "P"}: {}, {"S", "T"}: {},
	{"S", "K"}: {}, {"S", "M"}: {}, {"S", "N"}: {}, {"S", "L"}: {},
	{"S", "W"}: {}, {"SH", "R"}: {},
}

var arpabetToken = regexp.MustCompile(`^([A-Z]+)([0-2])?$`)

// arpabetToIPA converts one or more semicolon-separated ARPABET variants to
// slash-delimited IPA, joining variants with ", ".
func arpabetToIPA(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var converted []string
	for _, raw := range strings.Split(value, ";") {
		tokens := strings.Fields(strings.TrimSpace(raw))
		if len(tokens) == 0 {
			continue
		}
		converted = append(converted, singleARPABETToIPA(tokens))
	}
	return strings.Join(converted, ", ")
}

func singleARPABETToIPA(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}

	var result []string
	var consonants []string
	hasVowel := false

	for _, token := range tokens {
		match := arpabetToken.FindStringSubmatch(token)
		if match == nil {
			continue
		}
		phoneme, stress := match[1], match[2]

		if _, isVowel := ipaVowels[phoneme]; isVowel {
			vowel := phonemeToIPA[phoneme]
			if vowel == "" {
				vowel = phoneme
			}
			switch {
			case phoneme == "AH" && stress == "0":
				vowel = "ə"
			case phoneme == "ER" && stress == "0":
				vowel = "ər"
			case phoneme == "IY" && stress == "0":
				vowel = "i"
			case phoneme == "UW" && stress == "0":
				vowel = "u"
			}

			coda, onset := splitCodaAndOnset(consonants, hasVowel)
			for _, consonant := range coda {
				result = append(result, ipaFor(consonant))
			}
			switch stress {
			case "1":
				result = append(result, "ˈ")
			case "2":
				result = append(result, "ˌ")
			}
			for _, consonant := range onset {
				result = append(result, ipaFor(consonant))
			}
			result = append(result, vowel)
			consonants = nil
			hasVowel = true
			continue
		}
		consonants = append(consonants, phoneme)
	}

	for _, consonant := range consonants {
		result = append(result, ipaFor(consonant))
	}
	return "/" + strings.Join(result, "") + "/"
}

func splitCodaAndOnset(consonants []string, hasVowel bool) (coda, onset []string) {
	if !hasVowel || len(consonants) <= 1 {
		return nil, consonants
	}
	if len(consonants) == 2 {
		if _, valid := validOnsetPairs[[2]string{consonants[0], consonants[1]}]; valid {
			return nil, consonants
		}
		return consonants[:1], consonants[1:]
	}
	last := len(consonants)
	if _, valid := validOnsetPairs[[2]string{consonants[last-2], consonants[last-1]}]; valid {
		return consonants[:last-2], consonants[last-2:]
	}
	return consonants[:last-1], consonants[last-1:]
}

func ipaFor(phoneme string) string {
	if ipa, ok := phonemeToIPA[phoneme]; ok {
		return ipa
	}
	return phoneme
}

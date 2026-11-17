package dictionary

// Pronunciation holds the optional respelling and IPA columns.
type Pronunciation struct {
	Respelling string `json:"respelling,omitempty"`
	IPA        string `json:"ipa,omitempty"`
}

// Definition is one sense of an entry.
type Definition struct {
	PartOfSpeech string `json:"part_of_speech"`
	Text         string `json:"text"`
	Label        string `json:"label,omitempty"`
}

// Entry is a resolved dictionary headword.
type Entry struct {
	Word            string            `json:"word"`
	Transliteration string            `json:"transliteration,omitempty"`
	MatchedForm     string            `json:"matched_form,omitempty"`
	Pronunciation   Pronunciation     `json:"pronunciation"`
	Definitions     []Definition      `json:"definitions"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Info describes the dictionary database itself.
type Info struct {
	Name           string `json:"name"`
	SourceLanguage string `json:"source_language"`
	TargetLanguage string `json:"target_language"`
	License        string `json:"license,omitempty"`
	Attribution    string `json:"attribution,omitempty"`
}

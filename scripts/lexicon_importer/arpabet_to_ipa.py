"""ARPABET to IPA conversion module for Lexicon."""

import re

PHONEME_MAP = {
    'AA': 'ɑ', 'AE': 'æ', 'AH': 'ʌ', 'AO': 'ɔ', 'AW': 'aʊ',
    'AY': 'aɪ', 'B': 'b', 'CH': 'tʃ', 'D': 'd', 'DH': 'ð',
    'EH': 'ɛ', 'ER': 'ɜːr', 'EY': 'eɪ', 'F': 'f', 'G': 'ɡ',
    'HH': 'h', 'IH': 'ɪ', 'IY': 'iː', 'JH': 'dʒ', 'K': 'k',
    'L': 'l', 'M': 'm', 'N': 'n', 'NG': 'ŋ', 'OW': 'oʊ',
    'OY': 'ɔɪ', 'P': 'p', 'R': 'r', 'S': 's', 'SH': 'ʃ',
    'T': 't', 'TH': 'θ', 'UH': 'ʊ', 'UW': 'uː', 'V': 'v',
    'W': 'w', 'Y': 'j', 'Z': 'z', 'ZH': 'ʒ'
}

VOWELS = {
    'AA', 'AE', 'AH', 'AO', 'AW', 'AY', 'EH', 'ER',
    'EY', 'IH', 'IY', 'OW', 'OY', 'UH', 'UW'
}

VALID_ONSET_PAIRS = {
    ('P', 'R'), ('P', 'L'), ('B', 'R'), ('B', 'L'),
    ('T', 'R'), ('T', 'W'), ('D', 'R'), ('D', 'W'),
    ('K', 'R'), ('K', 'L'), ('K', 'W'), ('G', 'R'),
    ('G', 'L'), ('G', 'W'), ('F', 'R'), ('F', 'L'),
    ('TH', 'R'), ('TH', 'W'), ('S', 'P'), ('S', 'T'),
    ('S', 'K'), ('S', 'M'), ('S', 'N'), ('S', 'L'),
    ('S', 'W'), ('SH', 'R'),
}

def split_coda_and_onset(consonants: list[str], has_vowel: bool) -> tuple[list[str], list[str]]:
    if not has_vowel or len(consonants) <= 1:
        return [], consonants
    if len(consonants) == 2:
        if (consonants[0], consonants[1]) in VALID_ONSET_PAIRS:
            return [], consonants
        return consonants[:1], consonants[1:]
    if (consonants[-2], consonants[-1]) in VALID_ONSET_PAIRS:
        return consonants[:-2], consonants[-2:]
    return consonants[:-1], consonants[-1:]

def single_arpabet_to_ipa(tokens: list[str]) -> str:
    if not tokens:
        return ""
    result = []
    current_consonants = []
    has_vowel = False

    for token in tokens:
        m = re.match(r"^([A-Z]+)([0-2])?$", token)
        if not m:
            continue
        phoneme, stress = m.group(1), m.group(2)
        if phoneme in VOWELS:
            vowel_ipa = PHONEME_MAP.get(phoneme, phoneme)
            if phoneme == 'AH' and stress == '0':
                vowel_ipa = 'ə'
            elif phoneme == 'ER' and stress == '0':
                vowel_ipa = 'ər'
            elif phoneme == 'IY' and stress == '0':
                vowel_ipa = 'i'
            elif phoneme == 'UW' and stress == '0':
                vowel_ipa = 'u'

            coda, onset = split_coda_and_onset(current_consonants, has_vowel)
            for c in coda:
                result.append(PHONEME_MAP.get(c, c))

            if stress == '1':
                result.append("ˈ")
            elif stress == '2':
                result.append("ˌ")

            for o in onset:
                result.append(PHONEME_MAP.get(o, o))

            result.append(vowel_ipa)
            current_consonants = []
            has_vowel = True
        else:
            current_consonants.append(phoneme)

    for c in current_consonants:
        result.append(PHONEME_MAP.get(c, c))

    return f"/{''.join(result)}/"

def arpabet_to_ipa(arpabet_str: str) -> str:
    cleaned = arpabet_str.strip()
    if not cleaned:
        return ""
    variants = [v.strip().split() for v in cleaned.split(";") if v.strip()]
    ipa_variants = [single_arpabet_to_ipa(v) for v in variants if v]
    return ", ".join(ipa_variants)

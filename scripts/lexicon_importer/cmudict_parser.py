"""Parser for CMUdict formatted phonemes."""

import re
from collections import defaultdict
try:
    from .arpabet_to_ipa import arpabet_to_ipa
except ImportError:
    from arpabet_to_ipa import arpabet_to_ipa

def parse_cmudict(filepath: str) -> dict[str, str]:
    pronunciations = defaultdict(list)
    with open(filepath, 'r', encoding='latin-1') as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith(';;;'):
                continue
            parts = line.split()
            word_part = parts[0]
            phonemes = " ".join(parts[1:])
            # Clean word: e.g. "WORD(2)" -> "word"
            clean_word = re.sub(r'\(\d+\)$', '', word_part).lower()
            pronunciations[clean_word].append(phonemes)

    # Combine variants into single semicolon separated string and convert to IPA
    ipa_dict = {}
    for word, variant_list in pronunciations.items():
        combined_arpabet = " ; ".join(variant_list)
        ipa = arpabet_to_ipa(combined_arpabet)
        if ipa:
            ipa_dict[word] = ipa
    return ipa_dict

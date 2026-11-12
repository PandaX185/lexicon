# IPA pronunciation

Lexicon can show pronunciations as IPA (international phonetic alphabet) instead of a
respelling. Set `dictionary.pronunciation_style = "ipa"` to prefer IPA; dictionaries that
do not carry IPA fall back to the respelling field, and the popup simply hides the line
when neither is present.

```toml
[dictionary]
pronunciation_style = "ipa"
```

The conversion is done at import time, not at lookup time: the database stores the
finished IPA string in `entries.pronunciation_ipa`. `QmlPresenter` then fills the popup's
`pronunciation.text` with the preferred style, falling back to the other field when the
preferred one is empty. Nothing phonemic runs at display time.

## Source data

Pronunciations come from the CMU Pronouncing Dictionary 0.7b (`cmudict-0.7b.dict`),
copyright Carnegie Mellon University, distributed under the BSD 2-Clause license. The full
text is reproduced in `licenses/CMUDICT.txt`. CMUdict is North American English and lists
one or more ARPABET transcriptions per word; the importer keeps the first listing.

## Conversion

`scripts/lexicon_importer/arpabet_to_ipa.py` maps ARPABET phones to IPA and handles stress
and syllabification:

| ARPABET | IPA | ARPABET | IPA | ARPABET | IPA |
| --- | --- | --- | --- | --- | --- |
| AA | ɑ | B | b | CH | tʃ |
| AE | æ | D | d | DH | ð |
| AH | ʌ | F | f | G | ɡ |
| AO | ɔ | HH | h | JH | dʒ |
| AW | aʊ | K | k | L | l |
| AY | aɪ | M | m | N | n |
| EH | ɛ | NG | ŋ | P | p |
| ER | ɜːr | R | r | S | s |
| EY | eɪ | SH | ʃ | T | t |
| IH | ɪ | TH | θ | V | v |
| IY | iː | W | w | Y | j |
| OW | oʊ | Z | z | ZH | ʒ |
| OY | ɔɪ | UH | ʊ | UW | uː |

Rules applied on top of the table:

- Stress digits become marks: `1` → `ˈ` (primary), `2` → `ˌ` (secondary), `0` → no mark.
  The marks go immediately before the vowel.
- Unstressed vowels are reduced for diphthong-like qualities: unstressed `AH0` → `ə`,
  `ER0` → `ər`, `IY0` → `i`, `UW0` → `u`.
- Consonant clusters between vowels are split so the largest legal English onset starts
  the next syllable (a fixed list of onset pairs such as `str`, `pl`, `kw`), which keeps
  the stress mark on the right side of the consonant boundary.
- The result is wrapped in slashes, e.g. `serendipity` → `/ˌsɛrənˈdɪpɪti/`.

`tests/unit/test_arpabet_to_ipa.cpp` pins the mapping, stress placement, and onset splitting.

## Limitations

- CMUdict is North American English. British vowels and non-rhotic variants are not
  offered, and `ER` is always `ɜːr`/`ər`.
- Only listed headwords get a pronunciation. Inflected forms, proper nouns and many
  multi-word expressions are missing, and no grapheme-to-phoneme fallback exists, so
  those entries simply show no pronunciation line.
- Alternative pronunciations are discarded; only the first CMUdict transcription is kept.
- The respelling column of the bundled dictionary is intentionally empty.

A future release can fall back to Wiktionary pronunciations or an offline G2P model
(Epitran) for entries CMUdict does not cover. Neither is implemented, and any such source
must be bundled and downloaded explicitly by the user, never fetched during a lookup.

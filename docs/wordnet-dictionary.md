# Bundled WordNet dictionary

Lexicon ships `data/dictionaries/en.sqlite`, a read-only SQLite dictionary built from
Princeton WordNet 3.0 with pronunciations from the CMU Pronouncing Dictionary. It is
installed to `/usr/share/lexicon/dictionaries/en.sqlite` and is used for English lookups
without any network access.

## Provenance

| Item | Value |
| --- | --- |
| Definitions | Princeton WordNet 3.0 (`WNdb-3.0.tar.gz`, `dict/` data files) |
| Pronunciation | CMU Pronouncing Dictionary 0.7b (`cmudict-0.7b.dict`) |
| License | WordNet 3.0 license (`licenses/WORDNET.txt`), CMUdict BSD 2-Clause (`licenses/CMUDICT.txt`) |
| Attribution | “WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved.” |
| Entries | 147,806 headwords |
| Definitions | 206,918 senses |
| Inflected forms | 113,116 mappings |

The bundled file is generated, not hand-edited. It embeds its provenance in
`dictionary_info` and per-row `source`/`license` columns, so `--list-dictionaries` and the
catalog can identify it without a side file. Treat it as build output: never edit it in
place and never let the application write to it.

## Contents

- Headwords keep the WordNet spelling; lookup is case-insensitive.
- Every WordNet sense becomes one `definitions` row, with its `part_of_speech`
  (`noun`, `verb`, `adjective`, `adverb`) and a `sense_order` matching WordNet's own
  ordering. Duplicate senses for the same headword and part of speech are dropped.
- The WordNet synset lemmas other than the headword are stored as a single comma-separated
  `synonyms` row in `entry_metadata`.
- Function words and multi-word expressions are imported like any other entry; WordNet
  contains no example sentences.
- `pronunciation_respelling` is written as an empty string. Pronunciations live in
  `pronunciation_ipa` (see [pronunciation-ipa.md](pronunciation-ipa.md)).
- `transliteration` is empty. Transliteration for non-Latin dictionaries is expected to
  come from their own source data.
- `word_forms` maps inflected forms to their headwords: WordNet's `noun.exc`, `verb.exc`,
  `adj.exc`, and `adv.exc` lists supply irregulars (`went` to `go`, `children` to
  `child`), and conservative regular English rules add forms such as `walked` and
  `jumping`. A generated form that would resolve to more than one headword is omitted
  instead of picking one silently. Exact headword matches always win, so `running` still
  returns its own noun entry.

## Rebuilding the database

The importer lives in `scripts/lexicon_importer/import_wordnet.py` and has no third-party
dependencies beyond the Python standard library. Download the datasets yourself (they are
build-time inputs, never fetched by the application):

The full WordNet archive is required because the importer reads the `*.exc` exception
files. The smaller `WNdb-3.0.tar.gz` data-only archive does not contain them.

```sh
mkdir -p /tmp/wn && cd /tmp/wn
curl -LO https://wordnetcode.princeton.edu/3.0/WordNet-3.0.tar.gz
curl -LO https://raw.githubusercontent.com/cmusphinx/cmudict/master/cmudict.dict
tar xzf WordNet-3.0.tar.gz
cd -
```

Then regenerate the file:

```sh
python3 -m scripts.lexicon_importer.import_wordnet \
    --wordnet-dir /tmp/wn/WordNet-3.0/dict \
    --cmudict-file /tmp/wn/cmudict.dict \
    --schema data/dictionaries/schema.sql \
    --output data/dictionaries/en.sqlite
```

`just build-wordnet --wordnet-dir … --cmudict-file …` runs the same command. The importer
refuses to run without `--wordnet-dir`, warns when no CMUdict file is given, and writes
`ANALYZE`/`VACUUM`-ed output. The schema comes from `data/dictionaries/schema.sql`, so the
generated file always matches what `SqliteDictionaryRepository` expects.

## Testing

`tests/integration/test_wordnet_sqlite.cpp` opens the installed database and asserts that
`serendipity` has IPA, that `run` has both noun and verb senses, that inflected forms
such as `went` and `children` resolve to their headwords, and that the row counts are
plausible. It skips cleanly when the file is absent, so a checkout that has not generated
the dictionary still passes the rest of the suite.

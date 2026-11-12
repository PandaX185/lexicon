#!/usr/bin/env python3
"""Builds Lexicon's bundled English SQLite dictionary from Princeton WordNet 3.0.

WordNet supplies the headwords, senses, and glosses; the CMU Pronouncing
Dictionary supplies North American pronunciations, converted from ARPABET to
IPA. Both source datasets are build-time inputs only: the generated database is
what ships, and Lexicon itself never downloads anything.

Usage:
    python3 scripts/lexicon_importer/import_wordnet.py \\
        --wordnet-dir /tmp/wn/dict --cmudict-file /tmp/wn/cmudict.dict

Expected inputs: the `dict/` directory of WNdb-3.0.tar.gz (data.noun, data.verb,
data.adj, data.adv) and a CMUdict word/ARPABET text file.
"""

import argparse
import os
import re
import sqlite3
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from cmudict_parser import parse_cmudict

POS_BY_FILE = {
    'data.noun': 'noun',
    'data.verb': 'verb',
    'data.adj': 'adjective',
    'data.adv': 'adverb',
}

ATTRIBUTION = (
    'WordNet 3.0 Copyright 2006 by Princeton University. All rights reserved. '
    'Pronunciation data from the CMU Pronouncing Dictionary.'
)

MAX_SYNONYMS = 8


def parse_synsets(path: str, part_of_speech: str, words_per_synset: int = 0):
    """Yields (lemmas, definition) for every synset in a WordNet data file."""
    with open(path, 'r', encoding='utf-8', errors='replace') as handle:
        for line in handle:
            line = line.rstrip('\n')
            # The first 29 lines are the license header, indented with spaces.
            if not line or line.startswith('  '):
                continue
            header, separator, gloss = line.partition('|')
            if not separator:
                continue
            fields = header.split()
            if len(fields) < 5:
                continue
            try:
                word_count = int(fields[3], 16)
            except ValueError:
                continue

            lemmas = []
            index = 4
            for _ in range(word_count):
                if index >= len(fields):
                    break
                lemmas.append(fields[index].strip().replace('_', ' '))
                index += 2
            if not lemmas:
                continue

            yield lemmas, clean_gloss(gloss)


def clean_gloss(gloss: str) -> str:
    """Turns a WordNet gloss into one definition: the text before the examples.

    `"the quality of being ...; \"a rare example\""` loses the quoted examples,
    which the popup shows separately from the definition itself.
    """
    definition = gloss.split('"')[0].strip().rstrip(';').strip()
    if not definition:
        definition = gloss.strip().strip('"')
    if definition:
        definition = definition[0].upper() + definition[1:]
    return definition


def find_pos_files(wordnet_dir: str):
    files = []
    for name, part_of_speech in POS_BY_FILE.items():
        path = os.path.join(wordnet_dir, name)
        if os.path.exists(path):
            files.append((path, part_of_speech))
    return files


def build_database(wordnet_dir: str, cmudict_file: str, output_sqlite: str, schema_file: str) -> int:
    if not wordnet_dir:
        print("error: --wordnet-dir is required; extract WNdb-3.0.tar.gz and point at its dict/ directory",
              file=sys.stderr)
        return 2

    pos_files = find_pos_files(wordnet_dir)
    if not pos_files:
        print(f"error: no WordNet data files (data.noun, ...) found in {wordnet_dir}", file=sys.stderr)
        return 2

    cmudict = {}
    if cmudict_file:
        if not os.path.exists(cmudict_file):
            print(f"error: CMUdict file not found: {cmudict_file}", file=sys.stderr)
            return 2
        print(f"Loading CMUdict from {cmudict_file}...")
        cmudict = parse_cmudict(cmudict_file)
        print(f"  {len(cmudict)} pronunciations converted to IPA")
    else:
        print("warning: no CMUdict file given, writing entries without pronunciation", file=sys.stderr)

    entries: dict[str, dict] = {}

    def entry_for(word: str) -> dict:
        key = word.casefold()
        entry = entries.get(key)
        if entry is None:
            entry = {
                'word': word,
                'ipa': cmudict.get(key, ''),
                'senses': [],
                'synonyms': [],
            }
            entries[key] = entry
        return entry

    for path, part_of_speech in pos_files:
        count = 0
        for lemmas, definition in parse_synsets(path, part_of_speech):
            if not definition:
                continue
            for lemma in lemmas:
                entry = entry_for(lemma)
                entry['senses'].append((part_of_speech, definition))
                for sibling in lemmas:
                    if sibling.casefold() != lemma.casefold() and len(entry['synonyms']) < MAX_SYNONYMS:
                        if sibling not in entry['synonyms']:
                            entry['synonyms'].append(sibling)
            count += 1
        print(f"  {os.path.basename(path)}: {count} synsets")

    output_dir = os.path.dirname(os.path.abspath(output_sqlite))
    os.makedirs(output_dir, exist_ok=True)
    if os.path.exists(output_sqlite):
        os.remove(output_sqlite)

    connection = sqlite3.connect(output_sqlite)
    connection.execute('PRAGMA journal_mode = OFF')
    connection.execute('PRAGMA synchronous = OFF')
    cursor = connection.cursor()
    with open(schema_file, 'r', encoding='utf-8') as schema_handle:
        cursor.executescript(schema_handle.read())

    cursor.execute(
        """
        INSERT INTO dictionary_info (id, name, source_language, target_language, license, attribution)
        VALUES (1, 'WordNet English Dictionary', 'en', 'en', 'WordNet 3.0', ?)
        """,
        (ATTRIBUTION,),
    )

    print(f"Writing {len(entries)} headwords to {output_sqlite}...")
    entry_rows = []
    definition_rows = []
    metadata_rows = []
    entry_id = 0
    definition_id = 0

    for item in entries.values():
        # WordNet lists a lemma once per synset; the same gloss can still appear
        # through two entries of the same word, so drop exact duplicates.
        seen = set()
        senses = []
        for part_of_speech, definition in item['senses']:
            key = (part_of_speech, definition)
            if key in seen:
                continue
            seen.add(key)
            senses.append(key)
        if not senses:
            continue

        entry_id += 1
        entry_rows.append((entry_id, item['word'], item['ipa']))
        for sense_order, (part_of_speech, definition) in enumerate(senses):
            definition_id += 1
            definition_rows.append((definition_id, entry_id, part_of_speech, definition, sense_order))
        if item['synonyms']:
            metadata_rows.append((entry_id, 'synonyms', ', '.join(item['synonyms'])))

    cursor.executemany(
        """
        INSERT INTO entries (id, word, transliteration, pronunciation_respelling, pronunciation_ipa, source, license)
        VALUES (?, ?, '', '', ?, 'WordNet 3.0', 'WordNet 3.0')
        """,
        entry_rows,
    )
    cursor.executemany(
        """
        INSERT INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
        VALUES (?, ?, ?, ?, '', ?)
        """,
        definition_rows,
    )
    cursor.executemany("INSERT INTO entry_metadata (entry_id, key, value) VALUES (?, ?, ?)", metadata_rows)
    connection.commit()

    print("Optimizing database...")
    cursor.execute('ANALYZE')
    cursor.execute('VACUUM')
    connection.close()

    print(f"Wrote {entry_id} entries and {definition_id} definitions to {output_sqlite}")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Build Lexicon's WordNet SQLite dictionary")
    parser.add_argument('--wordnet-dir', default='', help="WordNet dict/ directory (containing data.noun, ...)")
    parser.add_argument('--cmudict-file', default='', help="CMUdict word/ARPABET text file")
    parser.add_argument('--schema', default='data/dictionaries/schema.sql', help="Path to schema.sql")
    parser.add_argument('--output', default='data/dictionaries/en.sqlite', help="Output SQLite database path")
    args = parser.parse_args()

    return build_database(args.wordnet_dir, args.cmudict_file, args.output, args.schema)


if __name__ == '__main__':
    sys.exit(main())

PRAGMA foreign_keys = ON;

INSERT OR IGNORE INTO entries (id, word, pronunciation_respelling, pronunciation_ipa, source, license)
VALUES
    (1, 'serendipity', 'ser-uhn-DIP-i-tee', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (2, 'lexicon', 'LEK-si-kon', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (3, 'run', 'run', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (4, 'apple', 'AP-uhl', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (5, 'bright', 'bryt', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (6, 'quickly', 'KWIK-lee', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (7, 'and', 'and', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (8, 'the', 'thuh', '', 'lexicon-dev-fixture', 'CC0-1.0'),
    (9, 'hello', 'heh-LOH', '', 'lexicon-dev-fixture', 'CC0-1.0');

INSERT OR IGNORE INTO definitions (id, entry_id, part_of_speech, definition, label, sense_order)
VALUES
    (1, 1, 'noun', 'The occurrence of a fortunate discovery by chance.', '', 0),
    (2, 2, 'noun', 'The vocabulary of a language, a person, or a field of knowledge.', '', 0),
    (3, 3, 'verb', 'To move quickly on foot, faster than walking.', '', 0),
    (4, 3, 'noun', 'An act or period of running.', '', 1),
    (5, 3, 'noun', 'A continuous sequence of performances, events, or operations.', 'figurative', 2),
    (6, 4, 'noun', 'The round fruit of a tree of the rose family.', '', 0),
    (7, 5, 'adjective', 'Emitting or reflecting a large amount of light.', '', 0),
    (8, 5, 'adjective', 'Quick to learn or understand.', 'figurative', 1),
    (9, 6, 'adverb', 'At a fast speed.', '', 0),
    (10, 7, 'conjunction', 'Used to connect words, phrases, or clauses.', '', 0),
    (11, 8, 'determiner', 'Used to refer to a specific person or thing already known.', '', 0),
    (12, 9, 'interjection', 'Used as a greeting or to begin a conversation.', '', 0);

INSERT OR IGNORE INTO entry_metadata (entry_id, key, value)
VALUES
    (1, 'fixture', 'true'),
    (2, 'fixture', 'true'),
    (3, 'fixture', 'true'),
    (4, 'fixture', 'true'),
    (5, 'fixture', 'true'),
    (6, 'fixture', 'true'),
    (7, 'fixture', 'true'),
    (8, 'fixture', 'true'),
    (9, 'fixture', 'true');

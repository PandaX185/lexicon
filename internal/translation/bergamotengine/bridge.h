#ifndef LEXICON_BERGAMOT_BRIDGE_H
#define LEXICON_BERGAMOT_BRIDGE_H

#ifdef __cplusplus
extern "C" {
#endif

// Opaque engine handle. Owned by Go, freed with lexicon_bergamot_destroy.
typedef struct lexicon_bergamot lexicon_bergamot;

// Creates a synchronous Bergamot service with one worker and no cache.
// Returns NULL and sets *error to an allocated string on failure.
lexicon_bergamot* lexicon_bergamot_create(char** error);

// Translates text using the model described by a Marian YAML configuration.
// Models are cached by configuration string. Returns an allocated UTF-8 string
// on success, or NULL with *error set. timeout_ms <= 0 means no explicit bound.
char* lexicon_bergamot_translate(lexicon_bergamot* engine, const char* model_config,
                                 const char* text, int timeout_ms, char** error);

// Destroys the engine. NULL is accepted.
void lexicon_bergamot_destroy(lexicon_bergamot* engine);

// Frees a string returned through this API. NULL is accepted.
void lexicon_bergamot_string_free(char* value);

#ifdef __cplusplus
}
#endif

#endif

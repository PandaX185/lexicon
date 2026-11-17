#include "bridge.h"

#include "translator/parser.h"
#include "translator/response.h"
#include "translator/service.h"

#include <chrono>
#include <cstring>
#include <exception>
#include <future>
#include <memory>
#include <mutex>
#include <new>
#include <string>
#include <unordered_map>

namespace {

char* copyString(const std::string& value) {
    char* result = new (std::nothrow) char[value.size() + 1];
    if (result == nullptr) {
        return nullptr;
    }
    std::memcpy(result, value.c_str(), value.size() + 1);
    return result;
}

void setError(char** error, const std::string& message) {
    if (error != nullptr) {
        *error = copyString(message);
    }
}

marian::bergamot::AsyncService::Config serviceConfig() {
    marian::bergamot::AsyncService::Config config;
    config.numWorkers = 1;
    config.cacheSize = 0;
    return config;
}

} // namespace

struct lexicon_bergamot {
    lexicon_bergamot() : service(serviceConfig()) {}

    marian::bergamot::AsyncService service;
    std::mutex modelsMutex;
    std::unordered_map<std::string, std::shared_ptr<marian::bergamot::TranslationModel>> models;
};

extern "C" lexicon_bergamot* lexicon_bergamot_create(char** error) {
    if (error != nullptr) {
        *error = nullptr;
    }
    try {
        auto* engine = new (std::nothrow) lexicon_bergamot();
        if (engine == nullptr) {
            setError(error, "could not allocate the translation engine");
        }
        return engine;
    } catch (const std::exception& exception) {
        setError(error, exception.what());
    } catch (...) {
        setError(error, "unknown error while creating the translation engine");
    }
    return nullptr;
}

extern "C" char* lexicon_bergamot_translate(lexicon_bergamot* engine, const char* model_config,
                                            const char* text, int timeout_ms, char** error) {
    if (error != nullptr) {
        *error = nullptr;
    }
    if (engine == nullptr) {
        setError(error, "translation engine is not initialized");
        return nullptr;
    }
    if (model_config == nullptr || text == nullptr || *text == '\0') {
        setError(error, "translation text is empty");
        return nullptr;
    }

    try {
        const std::string configuration(model_config);
        std::shared_ptr<marian::bergamot::TranslationModel> model;

        {
            std::scoped_lock lock(engine->modelsMutex);
            auto found = engine->models.find(configuration);
            if (found != engine->models.end()) {
                model = found->second;
            } else {
                auto options = marian::bergamot::parseOptionsFromString(configuration);
                model = engine->service.createCompatibleModel(options);
                if (!model) {
                    setError(error, "the translation model could not be created");
                    return nullptr;
                }
                engine->models.emplace(configuration, model);
            }
        }

        // The promise is shared because the engine may invoke the callback
        // after a timeout has already returned control to the caller.
        auto promise = std::make_shared<std::promise<marian::bergamot::Response>>();
        std::future<marian::bergamot::Response> future = promise->get_future();
        engine->service.translate(model, std::string(text),
                                  [promise](marian::bergamot::Response&& response) {
                                      promise->set_value(std::move(response));
                                  });

        if (timeout_ms > 0) {
            if (future.wait_for(std::chrono::milliseconds(timeout_ms)) != std::future_status::ready) {
                setError(error, "the translation timed out");
                return nullptr;
            }
        } else {
            future.wait();
        }

        const marian::bergamot::Response response = future.get();
        if (response.target.text.empty()) {
            setError(error, "the translator returned no text");
            return nullptr;
        }
        return copyString(response.target.text);
    } catch (const std::exception& exception) {
        setError(error, exception.what());
    } catch (...) {
        setError(error, "unknown error while translating");
    }
    return nullptr;
}

extern "C" void lexicon_bergamot_destroy(lexicon_bergamot* engine) {
    delete engine;
}

extern "C" void lexicon_bergamot_string_free(char* value) {
    delete[] value;
}

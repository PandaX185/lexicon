#include "core/selection/PrimarySelectionProvider.h"

#include <utility>

namespace Lexicon {

PrimarySelectionProvider::PrimarySelectionProvider(std::unique_ptr<PrimarySelectionSource> source,
                                                   SelectionTextPolicy policy)
    : m_source(std::move(source)), m_policy(std::move(policy)) {}

PrimarySelectionProvider::~PrimarySelectionProvider() = default;

SelectionResult PrimarySelectionProvider::currentSelection() {
    return m_policy.interpret(m_source->readPrimarySelection(m_policy.acceptedMimeTypes()));
}

QString PrimarySelectionProvider::displayName() const {
    return m_source->displayName();
}

} // namespace Lexicon

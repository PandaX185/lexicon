#pragma once

#include <QString>

namespace Lexicon {

struct WordEntry;

class PopupPresenter {
  public:
    virtual ~PopupPresenter() = default;

    virtual void showLoading(const QString& word) = 0;
    virtual void showEntry(const WordEntry& entry) = 0;
    virtual void showMessage(const QString& title, const QString& message) = 0;
    virtual void showSettings() = 0;
    virtual void dismiss() = 0;
};

} // namespace Lexicon

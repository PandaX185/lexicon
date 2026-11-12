#include "app/QmlPresenter.h"

#include "dictionary/models/WordEntry.h"
#include "infrastructure/logging/Logging.h"

#include <QCursor>
#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include <QQuickItem>
#include <QQuickView>
#include <QQuickWindow>
#include <QScreen>
#include <QSize>
#include <QVariantList>
#include <QVariantMap>

#include <algorithm>
#include <utility>

namespace {

QVariantMap entryToVariant(const Lexicon::WordEntry& entry, bool showPronunciation) {
    QVariantMap result;
    result.insert(QStringLiteral("word"), entry.word);

    QVariantMap pronunciation;
    pronunciation.insert(QStringLiteral("respelling"), entry.pronunciation.respelling);
    pronunciation.insert(QStringLiteral("ipa"), entry.pronunciation.ipa);
    result.insert(QStringLiteral("pronunciation"), pronunciation);

    QVariantList definitions;
    definitions.reserve(entry.definitions.size());
    for (const Lexicon::Definition& definition : entry.definitions) {
        QVariantMap item;
        item.insert(QStringLiteral("text"), definition.text);
        item.insert(QStringLiteral("partOfSpeech"), Lexicon::partOfSpeechToString(definition.partOfSpeech));
        item.insert(QStringLiteral("label"), definition.label);
        definitions.append(item);
    }
    result.insert(QStringLiteral("definitions"), definitions);

    QVariantMap metadata;
    for (auto it = entry.metadata.cbegin(); it != entry.metadata.cend(); ++it) {
        metadata.insert(it.key(), it.value());
    }
    result.insert(QStringLiteral("metadata"), metadata);

    if (!showPronunciation) {
        result.remove(QStringLiteral("pronunciation"));
    }
    return result;
}

} // namespace

namespace Lexicon {

QmlPresenter::QmlPresenter(QQmlApplicationEngine& engine, Config config)
    : m_engine(engine), m_config(std::move(config)) {}

QmlPresenter::~QmlPresenter() = default;

void QmlPresenter::ensurePopup() {
    if (m_popup) {
        return;
    }

    m_popup = std::make_unique<QQuickView>(&m_engine, nullptr);
    m_popup->setFlags(Qt::Tool | Qt::FramelessWindowHint | Qt::WindowStaysOnTopHint);
    m_popup->setColor(Qt::transparent);
    m_popup->setResizeMode(QQuickView::SizeViewToRootObject);
    m_popup->setSource(QUrl(QStringLiteral("qrc:/ui/popup/LookupPopup.qml")));

    if (m_popup->status() == QQuickView::Error) {
        qCWarning(lexiconUi) << "event=popup_load_failed errors=" << m_popup->errors();
        return;
    }

    if (QQuickItem* root = m_popup->rootObject()) {
        root->setProperty("preferredWidth", m_config.appearance.popupWidth);
        root->setProperty("animationDuration", m_config.appearance.animation == Animation::None ? 0 : 140);

        // `SizeViewToRootObject` only sizes the window when the source is set; the
        // popup changes size later (loading -> entry, user resize), so mirror the
        // root item's size onto the window ourselves.
        const auto syncWindowSize = [this, root] {
            m_popup->resize(QSize(static_cast<int>(root->width()), static_cast<int>(root->height())));
        };
        QObject::connect(root, &QQuickItem::widthChanged, m_popup.get(), syncWindowSize);
        QObject::connect(root, &QQuickItem::heightChanged, m_popup.get(), syncWindowSize);
    }
}

void QmlPresenter::setPopupProperty(const char* name, const QVariant& value) {
    if (!m_popup) {
        return;
    }
    if (QQuickItem* root = m_popup->rootObject()) {
        root->setProperty(name, value);
    }
}

void QmlPresenter::showLoading(const QString& word) {
    ensurePopup();
    setPopupProperty("loading", true);
    setPopupProperty("hasMessage", false);
    setPopupProperty("entry", QVariant());
    setPopupProperty("word", word);
    presentPopup();
}

void QmlPresenter::showEntry(const WordEntry& entry) {
    ensurePopup();
    setPopupProperty("loading", false);
    setPopupProperty("hasMessage", false);
    setPopupProperty("word", entry.word);
    setPopupProperty("entry", entryToVariant(entry, m_config.dictionary.showPronunciation));
    presentPopup();
}

void QmlPresenter::showMessage(const QString& title, const QString& message) {
    ensurePopup();
    setPopupProperty("loading", false);
    setPopupProperty("hasMessage", true);
    setPopupProperty("entry", QVariant());
    setPopupProperty("messageTitle", title);
    setPopupProperty("message", message);
    presentPopup();
}

void QmlPresenter::showSettings() {
    const QList<QObject*> roots = m_engine.rootObjects();
    if (roots.isEmpty()) {
        qCWarning(lexiconUi) << "event=settings_unavailable reason=no_root_object";
        return;
    }

    auto* window = qobject_cast<QQuickWindow*>(roots.constFirst());
    if (window == nullptr) {
        qCWarning(lexiconUi) << "event=settings_unavailable reason=root_is_not_a_window";
        return;
    }

    window->show();
    window->raise();
    window->requestActivate();
    qCInfo(lexiconUi) << "event=settings_shown";
}

void QmlPresenter::dismiss() {
    if (m_popup && m_popup->isVisible()) {
        m_popup->hide();
    }
}

namespace {

// The popup is placed on the screen under the cursor; fall back to the primary
// screen when the cursor is not on any screen (headless, mid-session changes).
QScreen* popupScreen() {
    if (QScreen* screen = QGuiApplication::screenAt(QCursor::pos())) {
        return screen;
    }
    return QGuiApplication::primaryScreen();
}

// Margin kept between the popup and the screen edges, so the resize grip and the
// popup border stay reachable.
constexpr int kScreenMargin = 20;

// On Wayland the compositor decides where a new window goes, so a popup that
// grows to the full work area can end up with its bottom edge clipped. Keep the
// automatic height to a comfortable fraction of the screen instead; the user can
// still drag the popup larger.
constexpr double kAutoHeightFraction = 0.7;

} // namespace

void QmlPresenter::applyScreenBounds() {
    QScreen* screen = popupScreen();
    if (screen == nullptr) {
        return;
    }

    const QRect available = screen->availableGeometry();
    const int maxHeight = std::max(0, available.height() - 2 * kScreenMargin);
    const int autoHeight =
        std::min(maxHeight, static_cast<int>(static_cast<double>(available.height()) * kAutoHeightFraction));

    setPopupProperty("maxPopupWidth", std::max(0, available.width() - 2 * kScreenMargin));
    setPopupProperty("maxPopupHeight", maxHeight);
    setPopupProperty("autoPopupHeight", autoHeight);
}

void QmlPresenter::positionPopup() {
    if (!m_popup) {
        return;
    }

    QScreen* screen = popupScreen();
    if (screen == nullptr) {
        return;
    }

    const QRect available = screen->availableGeometry();
    const QSize requested = QSize(m_config.appearance.popupWidth, 180);
    const QSize size = m_popup->size().expandedTo(requested);

    // If the popup is already larger than the available geometry, do not try to
    // place it with qBound(min, x, max) where max can be less than min.
    if (size.width() > available.width() || size.height() > available.height()) {
        qCWarning(lexiconUi) << "event=popup_too_large_for_screen"
                             << "width=" << size.width() << "height=" << size.height()
                             << "available_width=" << available.width() << "available_height=" << available.height();
        return;
    }

    const QPoint cursor = QCursor::pos();

    QPoint position;
    switch (m_config.appearance.position) {
    case PopupPosition::Center:
        position = available.center() - QPoint(size.width() / 2, size.height() / 2);
        break;
    case PopupPosition::Cursor:
    case PopupPosition::NearSelection:
        position = cursor + QPoint(16, 16);
        break;
    }

    const int maxX = std::max(available.left(), available.right() - size.width());
    const int maxY = std::max(available.top(), available.bottom() - size.height());

    position.setX(qBound(available.left(), position.x(), maxX));
    position.setY(qBound(available.top(), position.y(), maxY));
    m_popup->setPosition(position);
}

void QmlPresenter::presentPopup() {
    if (!m_popup || m_popup->rootObject() == nullptr) {
        return;
    }

    applyScreenBounds();
    positionPopup();
    m_popup->show();
    m_popup->raise();
    m_popup->requestActivate();
}

} // namespace Lexicon

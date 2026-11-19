// Hand-written; qutegen never overwrites *_manual files.
#include "cookiefilter_manual.h"

#include <QtWebEngineCore/QWebEngineCookieStore>

#include <memory>

extern "C" {
bool qute_webengine_cookie_filter(uintptr_t id, char *firstParty, int firstPartyLen, char *origin, int originLen, bool thirdParty);
void qute_webengine_cookie_filter_release(uintptr_t id);
}

namespace {
// FilterSlot owns the Go function's handle; Qt destroys the std::function,
// and with it the slot, when the filter is replaced or the store dies.
struct FilterSlot {
    uintptr_t id;
    explicit FilterSlot(uintptr_t i) : id(i) {}
    ~FilterSlot() { qute_webengine_cookie_filter_release(id); }
};
} // namespace

extern "C" void qute_webengine_set_cookie_filter(void *store, uintptr_t id) {
    auto *s = static_cast<QWebEngineCookieStore *>(store);
    if (id == 0) {
        s->setCookieFilter(nullptr);
        return;
    }
    s->setCookieFilter([slot = std::make_shared<FilterSlot>(id)](const QWebEngineCookieStore::FilterRequest &r) -> bool {
        // copied to UTF-8 here, on whatever thread Qt calls from; Go copies
        // the bytes again before this returns
        QByteArray fp = r.firstPartyUrl.toString().toUtf8();
        QByteArray o = r.origin.toString().toUtf8();
        return qute_webengine_cookie_filter(slot->id, fp.data(), int(fp.size()), o.data(), int(o.size()), r.thirdParty);
    });
}

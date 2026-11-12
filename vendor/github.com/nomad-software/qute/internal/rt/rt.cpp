#include "rt.h"

#include <QtCore/QCoreApplication>
#include <QtCore/QEvent>
#include <QtCore/QEventLoop>
#include <QtCore/QMetaObject>
#include <QtCore/QObject>
#include <QtCore/QThread>

#include <cstdlib>
#include <thread>
#include <unordered_set>
#include <vector>

extern "C" {
// Exported from Go (cgo.go).
void qute_rt_run(uintptr_t id);
void qute_rt_drain(void);
void qute_rt_destroyed(void *obj);
void qute_rt_visibility(void *obj, bool visible);
void qute_rt_reparent(void *obj, int what);
}

static std::thread::id qute_main_thread;

// Every QObject Go has wrapped and watches, so the event filter can ignore
// the rest without a call into Go. Touched only on the GUI thread.
static std::unordered_set<QObject *> qute_watched;

extern "C" void qute_rt_init(void) { qute_main_thread = std::this_thread::get_id(); }

extern "C" bool qute_rt_is_gui_thread(void) {
    QCoreApplication *app = QCoreApplication::instance();
    if (app) {
        return QThread::currentThread() == app->thread();
    }
    return std::this_thread::get_id() == qute_main_thread;
}

extern "C" bool qute_rt_app_exists(void) { return QCoreApplication::instance() != nullptr; }

extern "C" bool qute_rt_post(uintptr_t id) {
    QCoreApplication *app = QCoreApplication::instance();
    if (!app) {
        return false;
    }
    return QMetaObject::invokeMethod(app, [id] { qute_rt_run(id); }, Qt::QueuedConnection);
}

extern "C" bool qute_rt_post_drain(void) {
    QCoreApplication *app = QCoreApplication::instance();
    if (!app) {
        return false;
    }
    return QMetaObject::invokeMethod(app, [] { qute_rt_drain(); }, Qt::QueuedConnection);
}

extern "C" void qute_rt_watch(void *obj) {
    QObject *o = static_cast<QObject *>(obj);
    qute_watched.insert(o);
    // The context is a separate object that is never destroyed, so the
    // connection lives exactly as long as o does. Without one, o itself is
    // the receiver, and a destructor that disconnects its own receiver
    // connections before ~QObject emits destroyed (~QVideoSink does) would
    // drop the death notice and leave a dangling handle.
    static QObject *ctx = new QObject;
    QObject::connect(o, &QObject::destroyed, ctx, [o] {
        qute_watched.erase(o);
        qute_rt_destroyed(o);
    });
}

namespace {
// d_ptr is protected; a pointer to member taken through a subclass reads it
// on any QObject without a cast to a type the object is not.
struct DPeek : QObject {
    static QObjectData *of(QObject *o) { return (o->*(&DPeek::d_ptr)).data(); }
};
} // namespace

extern "C" bool qute_rt_dying(void *obj) { return DPeek::of(static_cast<QObject *>(obj))->wasDeleted; }

extern "C" bool qute_rt_has_parent(void *obj) { return static_cast<QObject *>(obj)->parent() != nullptr; }

extern "C" void *qute_rt_root(void *obj) {
    QObject *o = static_cast<QObject *>(obj);
    while (o->parent()) {
        o = o->parent();
    }
    return o;
}

static void qute_collect(QObject *o, std::vector<void *> &out) {
    if (qute_watched.count(o)) {
        out.push_back(o);
    }
    for (QObject *c : o->children()) {
        if (c) {
            qute_collect(c, out);
        }
    }
}

extern "C" void **qute_rt_subtree(void *obj, ptrdiff_t *n) {
    std::vector<void *> v;
    qute_collect(static_cast<QObject *>(obj), v);
    *n = ptrdiff_t(v.size());
    if (v.empty()) {
        return nullptr;
    }
    void **out = static_cast<void **>(std::malloc(v.size() * sizeof(void *)));
    for (size_t i = 0; i < v.size(); i++) {
        out[i] = v[i];
    }
    return out;
}

extern "C" const char *qute_rt_class_name(void *obj) {
    return static_cast<QObject *>(obj)->metaObject()->className();
}

extern "C" void qute_rt_delete_qobject(void *obj) { delete static_cast<QObject *>(obj); }

extern "C" void qute_rt_disconnect(void *conn) {
    auto *c = static_cast<QMetaObject::Connection *>(conn);
    QObject::disconnect(*c);
}

extern "C" void qute_rt_free_connection(void *conn) { delete static_cast<QMetaObject::Connection *>(conn); }

namespace {
// Watches every event of the application for two things the runtime must
// know. Show and Hide of a parentless widget or window pin it while it is on
// screen, so GC never deletes a window the user is looking at (spontaneous
// ones, sent when a window is minimised or restored, are ignored: the window
// is still open). A wrapped object that changes parent moves the Go state
// anchored in its tree (see anchor.go) and, given a parent by Qt, becomes
// Parented.
class QuteFilter : public QObject {
public:
    using QObject::QObject;
    bool eventFilter(QObject *o, QEvent *e) override {
        switch (e->type()) {
        case QEvent::Show:
        case QEvent::Hide:
            if (!e->spontaneous() && (o->isWidgetType() || o->isWindowType()) && !o->parent() &&
                qute_watched.count(o)) {
                qute_rt_visibility(o, e->type() == QEvent::Show);
            }
            break;
        case QEvent::ChildAdded:
        case QEvent::ChildRemoved: {
            QObject *c = static_cast<QChildEvent *>(e)->child();
            // A child under construction is not watched yet and has no
            // children; a child being destroyed has already left the set.
            if (c && (qute_watched.count(c) || !c->children().isEmpty())) {
                qute_rt_reparent(c, e->type() == QEvent::ChildAdded ? 1 : 0);
            }
            break;
        }
        case QEvent::ParentChange:
            if (qute_watched.count(o) || !o->children().isEmpty()) {
                qute_rt_reparent(o, 2);
            }
            break;
        default:
            break;
        }
        return false;
    }
};
} // namespace

extern "C" void qute_rt_install_filter(void) {
    static QCoreApplication *installed = nullptr;
    QCoreApplication *app = QCoreApplication::instance();
    if (!app || app == installed) {
        return;
    }
    installed = app;
    app->installEventFilter(new QuteFilter(app));
}

extern "C" void qute_rt_nested_loop(void) {
    if (QCoreApplication::instance()) {
        QEventLoop loop;
        loop.exec();
    }
}

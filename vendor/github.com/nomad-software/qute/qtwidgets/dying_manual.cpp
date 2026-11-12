// Hand-written; qutegen never overwrites *_manual files.
#include "dying_manual.h"

#include <QtWidgets/QWidget>

namespace {
// ~QWidget emits destroyed itself, before ~QObject sets wasDeleted, so the
// runtime's check of wasDeleted misses a dying widget. ~QWidget first sets
// QWidgetData::in_destructor, which is public, behind QWidget's private
// data pointer. An explicit instantiation may name a private member, so the
// pointer to it is taken there (the standard access idiom), not by a cast to
// a layout guessed from the header.
template <typename Tag, typename Tag::type M> struct qute_rob {
    friend typename Tag::type get(Tag) { return M; }
};
struct qute_widget_data {
    using type = QWidgetData *QWidget::*;
    friend type get(qute_widget_data);
};
template struct qute_rob<qute_widget_data, &QWidget::data>;
} // namespace

extern "C" bool qute_widget_in_destructor(void *obj) {
    QObject *o = static_cast<QObject *>(obj);
    if (!o->isWidgetType()) {
        return false;
    }
    return (static_cast<QWidget *>(o)->*get(qute_widget_data()))->in_destructor;
}

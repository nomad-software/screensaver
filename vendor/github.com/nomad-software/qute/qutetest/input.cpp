#include "input.h"

#include <QtCore/QString>
#include <QtTest/QTest>
#include <QtWidgets/QWidget>

extern "C" void qute_tools_click(void *widget, int button, int x, int y, bool at) {
    QWidget *w = static_cast<QWidget *>(widget);
    QPoint pos = at ? QPoint(x, y) : QPoint();
    QTest::mouseClick(w, Qt::MouseButton(button), Qt::NoModifier, pos);
}

extern "C" void qute_tools_dblclick(void *widget, int button) {
    QTest::mouseDClick(static_cast<QWidget *>(widget), Qt::MouseButton(button));
}

extern "C" void qute_tools_type(void *widget, const char *s, ptrdiff_t n) {
    QWidget *w = static_cast<QWidget *>(widget);
    const QString text = QString::fromUtf8(s, n);
    // QTest::keyClicks only handles ASCII; send other characters as text with
    // an unknown key code, which is what an input method does.
    for (qsizetype i = 0; i < text.size(); i++) {
        QChar c = text.at(i);
        QString one(c);
        if (c.isHighSurrogate() && i + 1 < text.size()) {
            one.append(text.at(++i));
        }
        if (c.unicode() < 128) {
            QTest::keyClick(w, char(c.unicode()));
        } else {
            QTest::sendKeyEvent(QTest::Click, w, Qt::Key_unknown, one, Qt::NoModifier);
        }
    }
}

extern "C" void qute_tools_key(void *widget, int key, int mods) {
    QTest::keyClick(static_cast<QWidget *>(widget), Qt::Key(key), Qt::KeyboardModifiers::fromInt(mods));
}

extern "C" void qute_tools_wait(int ms) { QTest::qWait(ms); }

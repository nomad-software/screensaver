// Hand-written; qutegen never overwrites *_manual files.
#include "application_manual.h"

#include <QtWidgets/QApplication>

#include <cstring>

namespace {
int qute_argc = 0;
char **qute_argv = nullptr;
} // namespace

extern "C" void *qute_application_new(const char *args, const ptrdiff_t *lens, ptrdiff_t n) {
    // QApplication keeps references to argc and argv for its whole life.
    qute_argc = int(n);
    qute_argv = new char *[n + 1];
    for (ptrdiff_t i = 0; i < n; i++) {
        qute_argv[i] = new char[lens[i] + 1];
        std::memcpy(qute_argv[i], args, lens[i]);
        qute_argv[i][lens[i]] = 0;
        args += lens[i];
    }
    qute_argv[n] = nullptr;
    // The runtime's event filter (visibility pins, reparenting) is installed
    // by rt.AppStarted, which qute.Main calls next.
    return static_cast<QApplication *>(new QApplication(qute_argc, qute_argv));
}

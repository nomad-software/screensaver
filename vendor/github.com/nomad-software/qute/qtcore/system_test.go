package qtcore_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qutetest"
)

func TestFileClasses(t *testing.T) {
	qutetest.Run(t, func() {
		td := qtcore.NewQTemporaryDir()
		defer td.Delete()
		if !td.IsValid() || td.FilePath("x") != filepath.Join(td.Path(), "x") {
			t.Fatalf("temporary dir: valid %v, path %q (%s)", td.IsValid(), td.Path(), td.ErrorString())
		}
		dir := td.Path()

		// QSaveFile writes nothing until Commit
		target := filepath.Join(dir, "saved.txt")
		sf := qtcore.NewQSaveFileWithName(target, nil)
		if !sf.Open(qtcore.QIODeviceBaseWriteOnly) {
			t.Fatal("QSaveFile.Open failed")
		}
		sf.Write([]byte("hello"))
		if _, err := os.Stat(target); err == nil {
			t.Error("QSaveFile wrote the target before Commit")
		}
		if !sf.Commit() {
			t.Error("Commit failed")
		}
		if b, _ := os.ReadFile(target); string(b) != "hello" {
			t.Errorf("committed %q", b)
		}
		sf.Delete()

		tf := qtcore.NewQTemporaryFileWithTemplateName(filepath.Join(dir, "tmp-XXXXXX"))
		if !tf.Open() || !strings.HasPrefix(filepath.Base(tf.FileName()), "tmp-") {
			t.Errorf("temporary file %q", tf.FileName())
		}
		name := tf.FileName()
		if !tf.AutoRemove() {
			t.Error("AutoRemove off by default")
		}
		tf.Delete()
		if _, err := os.Stat(name); err == nil {
			t.Error("temporary file not removed with its object")
		}
		if n := qtcore.QTemporaryFileCreateNativeFile(target); n != nil {
			t.Error("CreateNativeFile copied a file that is already native")
		}

		lf := qtcore.NewQLockFileWithFileName(filepath.Join(dir, "lock"))
		if !lf.TryLock() || !lf.IsLocked() {
			t.Fatal("TryLock failed")
		}
		ok, pid, _, _ := lf.GetLockInfo()
		if !ok || pid != int64(os.Getpid()) {
			t.Errorf("lock info: ok %v, pid %d, want %d", ok, pid, os.Getpid())
		}
		lf.Unlock()
		lf.Delete()

		os.MkdirAll(filepath.Join(dir, "+extra"), 0o755)
		os.WriteFile(filepath.Join(dir, "+extra", "conf"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "conf"), nil, 0o644)
		fs := qtcore.NewQFileSelector(nil)
		if got := fs.Select(filepath.Join(dir, "conf")); got != filepath.Join(dir, "conf") {
			t.Errorf("no selector: %q", got)
		}
		fs.SetExtraSelectors([]string{"extra"})
		if got := fs.Select(filepath.Join(dir, "conf")); got != filepath.Join(dir, "+extra", "conf") {
			t.Errorf("with selector: %q", got)
		}
		if !slices.Contains(fs.AllSelectors(), "extra") {
			t.Errorf("selectors %v", fs.AllSelectors())
		}
		fs.Delete()

		var seen []string
		it := qtcore.NewQDirIteratorWithPath(dir)
		for it.HasNext() {
			it.Next()
			seen = append(seen, it.FileName())
		}
		it.Delete()
		for _, want := range []string{"saved.txt", "conf", "+extra"} {
			if !slices.Contains(seen, want) {
				t.Errorf("iterator missed %q in %v", want, seen)
			}
		}

		w := qtcore.NewQFileSystemWatcher(nil)
		if !w.AddPath(dir) || !slices.Contains(w.Directories(), dir) {
			t.Fatalf("watching %v", w.Directories())
		}
		changed := ""
		w.OnDirectoryChanged(func(p string) { changed = p })
		os.WriteFile(filepath.Join(dir, "new"), nil, 0o644)
		if !waitFor(func() bool { return changed != "" }) || changed != dir {
			t.Errorf("directoryChanged %q, want %q", changed, dir)
		}
		w.Delete()

		si := qtcore.NewQStorageInfoWithPath(dir)
		if !si.IsValid() || si.BytesTotal() <= 0 || si.RootPath() == "" {
			t.Errorf("storage: valid %v, total %d, root %q", si.IsValid(), si.BytesTotal(), si.RootPath())
		}
		if !qtcore.QStorageInfoRoot().IsRoot() {
			t.Error("QStorageInfoRoot is not the root")
		}

		r := qtcore.NewQResource()
		r.SetFileName(":/qute/does-not-exist")
		if r.IsValid() {
			t.Error("a missing resource is valid")
		}
		r.Delete()
	})
}

func TestStandardPathsAndLibraryInfo(t *testing.T) {
	qutetest.Run(t, func() {
		if qtcore.QStandardPathsWritableLocation(qtcore.QStandardPathsTempLocation) == "" {
			t.Error("no temp location")
		}
		if qtcore.QStandardPathsFindExecutable("sh") == "" {
			t.Error("sh not found")
		}
		if qtcore.QStandardPathsDisplayName(qtcore.QStandardPathsHomeLocation) == "" {
			t.Error("no display name for home")
		}
		qtcore.QStandardPathsSetTestModeEnabled(true)
		if !qtcore.QStandardPathsIsTestModeEnabled() {
			t.Error("test mode not enabled")
		}
		qtcore.QStandardPathsSetTestModeEnabled(false)

		v := qtcore.QLibraryInfoVersion()
		if v.MajorVersion() != 6 || v.MinorVersion() < 12 {
			t.Errorf("Qt version %s, want 6.12 or newer", v.ToString())
		}
		if qtcore.QLibraryInfoPath(qtcore.QLibraryInfoPluginsPath) == "" || qtcore.QLibraryInfoBuild() == "" {
			t.Error("no plugins path or build string")
		}
		if !qtcore.QLibraryInfoIsSharedBuild() {
			t.Error("Qt is not a shared build")
		}

		n := qtcore.QVersionNumberFromString("6.12.0")
		if !slices.Equal(n.Segments(), []int{6, 12, 0}) || n.SegmentAt(1) != 12 || n.SegmentCount() != 3 {
			t.Errorf("segments %v", n.Segments())
		}
		if n.Normalized().ToString() != "6.12" || !n.Normalized().IsNormalized() {
			t.Errorf("normalized %q", n.Normalized().ToString())
		}
		six := qtcore.NewQVersionNumberWithMaj(6)
		if !six.IsPrefixOf(n) || qtcore.QVersionNumberCompare(six, n) >= 0 {
			t.Error("6 is not a prefix of, and older than, 6.12.0")
		}
		if qtcore.QVersionNumberCommonPrefix(n, qtcore.NewQVersionNumberWithMajMinMic(6, 12, 3)).ToString() != "6.12" {
			t.Error("common prefix")
		}
		if !qtcore.NewQVersionNumber().IsNull() {
			t.Error("default version is not null")
		}

		mac := qtcore.NewQOperatingSystemVersionWithOSType(qtcore.QOperatingSystemVersionMacOS, 14)
		if mac.MajorVersion() != 14 || mac.Name() != "macOS" || mac.Type() != qtcore.QOperatingSystemVersionBaseMacOS {
			t.Errorf("macOS 14: %d %q", mac.MajorVersion(), mac.Name())
		}
		base := qtcore.NewQOperatingSystemVersionBaseWithOSType(qtcore.QOperatingSystemVersionBaseWindows, 10)
		if qtcore.QOperatingSystemVersionBaseName(base) != "Windows" {
			t.Errorf("base name %q", qtcore.QOperatingSystemVersionBaseName(base))
		}
		ux := qtcore.NewQOperatingSystemVersionUnexportedWithOperatingSystemVersionBase(base)
		if ux.MajorVersion() != 10 {
			t.Errorf("unexported major %d", ux.MajorVersion())
		}
		if qtcore.NewQOperatingSystemVersionWithOperatingSystemVersionBase(base).Type() != qtcore.QOperatingSystemVersionBaseWindows {
			t.Error("conversion from the base lost its type")
		}
		if qtcore.QOperatingSystemVersionCurrentType() != qtcore.QOperatingSystemVersionUnknown {
			t.Errorf("current type on Linux: %v, want Unknown", qtcore.QOperatingSystemVersionCurrentType())
		}

		if !qtcore.NewQTypeRevision().IsValid() == false || !qtcore.QTypeRevisionZero().IsValid() || qtcore.QTypeRevisionZero().MajorVersion() != 0 {
			t.Error("type revisions")
		}
	})
}

func TestProcess(t *testing.T) {
	qutetest.Run(t, func() {
		env := qtcore.QProcessEnvironmentSystemEnvironment()
		env.Insert("QUTE_TEST_VALUE", "42")
		if !env.Contains("QUTE_TEST_VALUE") || env.Value("QUTE_TEST_VALUE") != "42" || !slices.Contains(env.Keys(), "QUTE_TEST_VALUE") {
			t.Error("environment insert")
		}

		p := qtcore.NewQProcess(nil)
		p.SetProgram("sh")
		p.SetArguments([]string{"-c", "echo $QUTE_TEST_VALUE; echo oops >&2; exit 3"})
		p.SetProcessEnvironment(env)
		params := qtcore.NewQProcessUnixProcessParameters()
		params.SetFlags(qtcore.QProcessUnixProcessFlagResetSignalHandlers)
		p.SetUnixProcessParameters(params)
		if p.UnixProcessParameters().Flags() != qtcore.QProcessUnixProcessFlagResetSignalHandlers {
			t.Errorf("unix parameters %v", p.UnixProcessParameters().Flags())
		}
		p.Start()
		if !p.WaitForFinished() {
			t.Fatalf("process did not finish: %v", p.Error())
		}
		if got := strings.TrimSpace(string(p.ReadAllStandardOutput())); got != "42" {
			t.Errorf("stdout %q", got)
		}
		if got := strings.TrimSpace(string(p.ReadAllStandardError())); got != "oops" {
			t.Errorf("stderr %q", got)
		}
		if p.ExitCode() != 3 || p.ExitStatus() != qtcore.QProcessNormalExit {
			t.Errorf("exit %d, %v", p.ExitCode(), p.ExitStatus())
		}
		p.Delete()

		// a pipeline: the first process writes into the second
		from := qtcore.NewQProcess(nil)
		to := qtcore.NewQProcess(nil)
		from.SetStandardOutputProcess(to)
		from.SetProgram("echo")
		from.SetArguments([]string{"piped"})
		to.SetProgram("tr")
		to.SetArguments([]string{"a-z", "A-Z"})
		from.Start()
		to.Start()
		from.WaitForFinished()
		to.WaitForFinished()
		if got := strings.TrimSpace(string(to.ReadAllStandardOutput())); got != "PIPED" {
			t.Errorf("pipeline output %q", got)
		}
		from.Delete()
		to.Delete()

		if qtcore.QProcessExecute("true") != 0 {
			t.Error("QProcessExecute(true) failed")
		}
	})
}

func TestCommandLineParser(t *testing.T) {
	qutetest.Run(t, func() {
		p := qtcore.NewQCommandLineParser()
		defer p.Delete()
		p.SetApplicationDescription("test")
		verbose := qtcore.NewQCommandLineOptionWithNamesDescription([]string{"v", "verbose"}, "say more")
		out := qtcore.NewQCommandLineOptionWithNamesDescription([]string{"o", "output"}, "where to write")
		out.SetValueName("file")
		out.SetDefaultValue("out.txt")
		if !p.AddOption(verbose) || !p.AddOption(out) {
			t.Fatal("AddOption failed")
		}
		p.AddPositionalArgument("input", "what to read")
		if !p.Parse([]string{"app", "-v", "in.txt"}) {
			t.Fatalf("parse: %s", p.ErrorText())
		}
		if !p.IsSet("verbose") || !p.IsSetCommandLineOption(verbose) || p.Value("output") != "out.txt" {
			t.Errorf("verbose %v, output %q", p.IsSet("verbose"), p.Value("output"))
		}
		if !slices.Equal(p.PositionalArguments(), []string{"in.txt"}) {
			t.Errorf("positional %v", p.PositionalArguments())
		}
		if !strings.Contains(p.HelpText(), "--output <file>") {
			t.Errorf("help text:\n%s", p.HelpText())
		}
		if p.Parse([]string{"app", "--bogus"}) || p.ErrorText() == "" || !slices.Equal(p.UnknownOptionNames(), []string{"bogus"}) {
			t.Errorf("unknown option: error %q, unknown %v", p.ErrorText(), p.UnknownOptionNames())
		}
		if !slices.Equal(out.Names(), []string{"o", "output"}) || out.ValueName() != "file" || out.Description() != "where to write" {
			t.Error("option getters")
		}
	})
}

func TestLoggingAndMime(t *testing.T) {
	qutetest.Run(t, func() {
		c := qtcore.NewQLoggingCategoryWithCategory("qute.test")
		if c.CategoryName() != "qute.test" {
			t.Errorf("name %q", c.CategoryName())
		}
		qtcore.QLoggingCategorySetFilterRules("qute.test.debug=false")
		if c.IsDebugEnabled() || !c.IsWarningEnabled() {
			t.Error("debug should be off, warnings on")
		}
		qtcore.QLoggingCategorySetFilterRules("qute.test.debug=true")
		if !c.IsDebugEnabled() {
			t.Error("debug should be on")
		}
		qtcore.QLoggingCategorySetFilterRules("")
		c.Delete()
		if qtcore.QLoggingCategoryDefaultCategory().CategoryName() != "default" {
			t.Error("default category")
		}

		db := qtcore.NewQMimeDatabase()
		defer db.Delete()
		plain := db.MimeTypeForName("text/plain")
		if !plain.IsValid() || plain.Name() != "text/plain" || !slices.Contains(plain.Suffixes(), "txt") {
			t.Errorf("text/plain: valid %v, suffixes %v", plain.IsValid(), plain.Suffixes())
		}
		if got := db.MimeTypeForData([]byte("%PDF-1.4\n")).Name(); got != "application/pdf" {
			t.Errorf("PDF data: %q", got)
		}
		if got := db.MimeTypeForFile("notes.txt").Name(); got != "text/plain" {
			t.Errorf("notes.txt: %q", got)
		}
		buf := qtcore.NewQBuffer(nil)
		buf.SetData([]byte("<?xml version=\"1.0\"?><a/>"))
		buf.Open(qtcore.QIODeviceBaseReadOnly)
		if got := db.MimeTypeForDataIODevice(buf); !got.Inherits("text/plain") {
			t.Errorf("XML from a device: %q does not inherit text/plain", got.Name())
		}
		buf.Delete()
		if db.SuffixForFileName("a.tar.gz") != "tar.gz" {
			t.Errorf("suffix %q", db.SuffixForFileName("a.tar.gz"))
		}
	})
}

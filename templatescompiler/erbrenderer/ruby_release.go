package erbrenderer

import (
	"path"
	"runtime"
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

// RubySearchGlob is the pattern used to search for the ruby executable
const RubySearchGlob string = "ruby_*_cpi/bin/ruby"

// RubySearchGlobWindows is the pattern used to search for the ruby executable on Windows
const RubySearchGlobWindows string = RubySearchGlob + ".exe"

// RubyRelease is used to locate the installed ruby that comes with the CPI
type RubyRelease interface {
	// ExecutablePath is the full path to the executable ruby or ruby.exe binary
	ExecutablePath() string

	// BinDir is the full path to the directory which contains the ruby binary
	BinDir() string
}

type rubyRelease struct {
	boshPackageDir string
	fs             boshsys.FileSystem
	logger         boshlog.Logger
	logTag         string
	executablePath string
	binDir         string
}

// NewRubyRelease creates a new RubyRelease
func NewRubyRelease(
	boshPackageDir string,
	fs boshsys.FileSystem,
	logger boshlog.Logger,
) RubyRelease {
	return &rubyRelease{
		boshPackageDir: boshPackageDir,
		fs:             fs,
		logger:         logger,
		logTag:         "rubyRelease",
	}
}

func (r *rubyRelease) BinDir() string {
	if len(r.binDir) == 0 {
		r.binDir = r.findBinDir()
	}
	return r.binDir
}

func (r *rubyRelease) ExecutablePath() string {
	if len(r.executablePath) == 0 {
		r.executablePath = r.findExecutablePath()
	}
	return r.executablePath
}

func (r *rubyRelease) findBinDir() string {
	rubyExe := r.ExecutablePath()
	if rubyExe == "ruby" {
		return ""
	}
	return path.Dir(rubyExe)
}

func (r *rubyRelease) findExecutablePath() string {
	matches, err := r.fs.Glob(path.Join(r.boshPackageDir, r.rubySearchString()))
	if err != nil || len(matches) == 0 {
		r.logger.Debug(r.logTag, "Couldn't find the ruby bundled with the cpi release, defaulting to system ruby")
		return "ruby"
	}
	return strings.Replace(matches[0], "\\", "/", -1)
}

func (r *rubyRelease) rubySearchString() string {
	if runtime.GOOS == "windows" {
		return RubySearchGlobWindows
	}
	return RubySearchGlob
}

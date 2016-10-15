package erbrenderer

import (
	"path/filepath"
	"runtime"
	"strings"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

// RubyReleaseFinder is used to locate the installed ruby that comes with the CPI
type RubyReleaseFinder interface {
	RubyExecutable() string
	RubyDir() string
}

type rubyReleaseFinder struct {
	boshPackageDir string
	logger         boshlog.Logger
	logTag         string
}

// NewRubyReleaseFinder creates a new RubyReleaseFinder
func NewRubyReleaseFinder(
	boshPackageDir string,
	logger boshlog.Logger,
) RubyReleaseFinder {
	return rubyReleaseFinder{
		boshPackageDir: boshPackageDir,
		logger:         logger,
		logTag:         "rubyReleaseFinder",
	}
}

func (r rubyReleaseFinder) RubyDir() string {
	return strings.Replace(filepath.Dir(r.RubyExecutable()), "\\", "/", -1)
}

func (r rubyReleaseFinder) RubyExecutable() string {
	matches, err := filepath.Glob(filepath.Join(r.boshPackageDir, r.rubySearchString()))
	if err != nil || len(matches) == 0 {
		r.logger.Debug(r.logTag, "Couldn't find the ruby bundled with the cpi release, defaulting to system ruby")
		return "ruby"
	}
	return matches[0]
}

func (r rubyReleaseFinder) rubySearchString() string {
	if runtime.GOOS == "windows" {
		return "ruby_*_cpi/bin/ruby.exe"
	}
	return "ruby_*_cpi/bin/ruby"
}

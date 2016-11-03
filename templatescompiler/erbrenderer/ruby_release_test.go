package erbrenderer_test

import (
	. "github.com/cloudfoundry/bosh-init/templatescompiler/erbrenderer"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("RubyRelease", func() {
	const boshPkgDir string = "/tmp/foo/"
	var (
		fs          *fakesys.FakeFileSystem
		rubyRelease RubyRelease
	)

	BeforeEach(func() {
		logger := boshlog.NewLogger(boshlog.LevelNone)
		fs = fakesys.NewFakeFileSystem()

		rubyRelease = NewRubyRelease(boshPkgDir, fs, logger)
	})

	Context("ruby not found in CPI release", func() {
		BeforeEach(func() {
			fs.SetGlob(boshPkgDir+RubySearchGlob, []string{})
		})

		It("returns system ruby executable", func() {
			Expect(rubyRelease.ExecutablePath()).To(Equal("ruby"))
		})

		It("returns empty string for dir", func() {
			Expect(rubyRelease.BinDir()).To(Equal(""))
		})
	})

	Context("ruby found in CPI release", func() {
		const cpiRubyExe string = boshPkgDir + "ruby_aws_cpi/bin/ruby"
		BeforeEach(func() {
			fs.SetGlob(boshPkgDir+RubySearchGlob, []string{cpiRubyExe})
		})

		It("returns full path to ruby executable", func() {
			Expect(rubyRelease.ExecutablePath()).To(Equal(cpiRubyExe))
		})

		It("returns ruby dir", func() {
			Expect(rubyRelease.BinDir()).To(Equal(boshPkgDir + "ruby_aws_cpi/bin"))
		})

		It("returns ruby dir cached", func() {
			binDir := rubyRelease.BinDir()                    // this causes the result to be cached
			fs.SetGlob(boshPkgDir+RubySearchGlob, []string{}) // clear glob result
			Expect(rubyRelease.BinDir()).To(Equal(binDir))
		})
	})

	Context("ruby found in CPI release with backslashes in path", func() {
		const cpiRubyExe string = boshPkgDir + "ruby_aws_cpi\\bin\\ruby"
		BeforeEach(func() {
			fs.SetGlob(boshPkgDir+RubySearchGlob, []string{cpiRubyExe})
		})

		It("returns full path without any backslashes", func() {
			Expect(rubyRelease.ExecutablePath()).NotTo(ContainSubstring("\\"))
		})

		It("returns dir without any backslashes", func() {
			Expect(rubyRelease.BinDir()).NotTo(ContainSubstring("\\"))
		})
	})
})
